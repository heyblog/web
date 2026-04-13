const DEFAULT_WEB_HOST = '127.0.0.1';
const DEFAULT_WEB_PORT = 9101;

export const getWebPort = (): number => {
  const raw = process.env.PLAYWRIGHT_WEB_PORT ?? process.env.PORT ?? String(DEFAULT_WEB_PORT);
  const port = Number(raw);

  return Number.isFinite(port) && port > 0 ? port : DEFAULT_WEB_PORT;
};

export const getWebBaseUrl = (): string =>
  process.env.PLAYWRIGHT_WEB_BASE_URL ?? `http://${DEFAULT_WEB_HOST}:${getWebPort()}`;
