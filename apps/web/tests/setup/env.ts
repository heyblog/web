const DEFAULT_WEB_HOST = '127.0.0.1';
const DEFAULT_API_HOST = '127.0.0.1';
const DEFAULT_WEB_PORT = 9101;
const DEFAULT_API_PORT = 9201;

export const getWebPort = (): number => {
  const raw = process.env.PLAYWRIGHT_WEB_PORT ?? process.env.PORT ?? String(DEFAULT_WEB_PORT);
  const port = Number(raw);

  return Number.isFinite(port) && port > 0 ? port : DEFAULT_WEB_PORT;
};

export const getWebBaseUrl = (): string =>
  process.env.PLAYWRIGHT_WEB_BASE_URL ?? `http://${DEFAULT_WEB_HOST}:${getWebPort()}`;

export const getApiBaseUrl = (): string =>
  process.env.WEB_API_BASE_URL ?? `http://${DEFAULT_API_HOST}:${DEFAULT_API_PORT}`;
