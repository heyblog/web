import { mkdirSync } from 'node:fs';
import { join } from 'node:path';

import type pino from 'pino';

const DEFAULT_LOG_DIR = join(process.cwd(), 'logs');

type ApiLoggerConfig = {
  NODE_ENV?: string;
  API_LOG_LEVEL?: string;
  API_LOG_DIR?: string;
};

const resolveLogLevel = (config: ApiLoggerConfig): string => {
  if (config.API_LOG_LEVEL) {
    return config.API_LOG_LEVEL;
  }

  const env = config.NODE_ENV ?? 'development';
  if (env === 'production') {
    return 'info';
  }

  return 'debug';
};

export const resolveApiLogFilePath = (config: ApiLoggerConfig): string => {
  const env = config.NODE_ENV ?? 'development';
  const logDir = config.API_LOG_DIR ?? DEFAULT_LOG_DIR;
  mkdirSync(logDir, { recursive: true });
  return join(logDir, `api-${env}.log`);
};

export function getLoggerOptions(config: ApiLoggerConfig): pino.LoggerOptions {
  const env = config.NODE_ENV ?? 'development';
  const level = resolveLogLevel(config);
  const logFilePath = resolveApiLogFilePath(config);

  if (env === 'production') {
    return {
      level,
      errorKey: 'error',
      transport: {
        targets: [
          {
            target: 'pino/file',
            options: { destination: 1 },
          },
          {
            target: 'pino/file',
            options: {
              destination: logFilePath,
              mkdir: true,
              append: true,
            },
          },
        ],
      },
    };
  }

  return {
    level,
    errorKey: 'error',
    transport: {
      targets: [
        {
          target: 'pino-pretty',
          options: {
            translateTime: 'SYS:standard',
            singleLine: true,
            ignore: 'pid,hostname',
            errorLikeObjectKeys: ['error', 'err'],
          },
        },
        {
          target: 'pino/file',
          options: {
            destination: logFilePath,
            mkdir: true,
            append: true,
          },
        },
      ],
    },
  };
}
