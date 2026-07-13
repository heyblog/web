import net from 'node:net';
import tls from 'node:tls';

import type { AppConfig } from '@/infrastructure/app/http/app-config.service';

import type { MailMessage } from './mail.types';
import { buildMimeMessage } from './mail-message.service';

type SocketLike = net.Socket | tls.TLSSocket;

const CRLF = '\r\n';
const LOCAL_NAME = 'localhost';
const SMTP_TIMEOUT_MS = 10000;

const readSmtpResponse = (socket: SocketLike): Promise<string> =>
  new Promise((resolve, reject) => {
    let buffer = '';

    const cleanup = () => {
      socket.off('data', onData);
      socket.off('error', onError);
      socket.off('close', onClose);
      socket.off('timeout', onTimeout);
    };

    const onError = (error: Error) => {
      cleanup();
      reject(error);
    };

    const onClose = () => {
      cleanup();
      reject(new Error('SMTP socket closed unexpectedly'));
    };

    const onTimeout = () => {
      cleanup();
      reject(new Error('SMTP socket timed out'));
    };

    const onData = (chunk: Buffer) => {
      buffer += chunk.toString('utf8');
      const lines = buffer.split(/\r?\n/).filter(Boolean);
      const lastLine = lines.at(-1);

      if (!lastLine || !/^\d{3}[ -]/.test(lastLine)) {
        return;
      }

      if (lastLine[3] === '-') {
        return;
      }

      cleanup();
      resolve(buffer);
    };

    socket.on('data', onData);
    socket.on('error', onError);
    socket.on('close', onClose);
    socket.on('timeout', onTimeout);
  });

const expectResponse = async (socket: SocketLike, expectedPrefix: string): Promise<string> => {
  const response = await readSmtpResponse(socket);

  if (!response.startsWith(expectedPrefix)) {
    throw new Error(`unexpected SMTP response: ${response.trim()}`);
  }

  return response;
};

const sendCommand = async (
  socket: SocketLike,
  command: string,
  expectedPrefix: string,
): Promise<string> => {
  socket.write(`${command}${CRLF}`);
  return expectResponse(socket, expectedPrefix);
};

const createSocket = (config: AppConfig): Promise<SocketLike> =>
  new Promise((resolve, reject) => {
    const handleConnect = () => {
      socket.setTimeout(SMTP_TIMEOUT_MS);
      resolve(socket);
    };

    const socket = config.API_SMTP_SECURE
      ? tls.connect(
          {
            host: config.API_SMTP_HOST,
            port: config.API_SMTP_PORT,
            servername: config.API_SMTP_HOST,
          },
          handleConnect,
        )
      : net.createConnection(
          {
            host: config.API_SMTP_HOST,
            port: config.API_SMTP_PORT,
          },
          handleConnect,
        );

    socket.once('error', reject);
  });

async function maybeUpgradeStartTls(
  socket: SocketLike,
  config: AppConfig,
  ehloResponse: string,
): Promise<SocketLike> {
  if (config.API_SMTP_SECURE || !/\bSTARTTLS\b/i.test(ehloResponse)) {
    return socket;
  }

  await sendCommand(socket, 'STARTTLS', '220');

  const upgraded = await new Promise<tls.TLSSocket>((resolve, reject) => {
    const tlsSocket = tls.connect(
      {
        socket,
        servername: config.API_SMTP_HOST,
      },
      () => resolve(tlsSocket),
    );

    tlsSocket.once('error', reject);
  });

  upgraded.setTimeout(SMTP_TIMEOUT_MS);
  return upgraded;
}

async function authenticate(socket: SocketLike, config: AppConfig): Promise<void> {
  if (!config.API_SMTP_USER || !config.API_SMTP_PASS) {
    return;
  }

  const authToken = Buffer.from(
    `\u0000${config.API_SMTP_USER}\u0000${config.API_SMTP_PASS}`,
    'utf8',
  ).toString('base64');

  await sendCommand(socket, `AUTH PLAIN ${authToken}`, '235');
}

export async function sendMailThroughSmtp(config: AppConfig, message: MailMessage): Promise<void> {
  let socket: SocketLike | null = null;

  try {
    socket = await createSocket(config);
    await expectResponse(socket, '220');

    let ehloResponse = await sendCommand(socket, `EHLO ${LOCAL_NAME}`, '250');
    socket = await maybeUpgradeStartTls(socket, config, ehloResponse);

    if (!config.API_SMTP_SECURE && /\bSTARTTLS\b/i.test(ehloResponse)) {
      ehloResponse = await sendCommand(socket, `EHLO ${LOCAL_NAME}`, '250');
    }

    void ehloResponse;

    await authenticate(socket, config);
    await sendCommand(socket, `MAIL FROM:<${config.API_SMTP_FROM}>`, '250');
    await sendCommand(socket, `RCPT TO:<${message.to}>`, '250');
    await sendCommand(socket, 'DATA', '354');
    socket.write(`${buildMimeMessage(config, message)}${CRLF}.${CRLF}`);
    await expectResponse(socket, '250');
    await sendCommand(socket, 'QUIT', '221');
  } finally {
    socket?.destroy();
  }
}
