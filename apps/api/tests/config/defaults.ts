export const TEST_CONFIG = {
  NODE_ENV: 'test',
  DATABASE_URL: 'postgres://postgres:postgres@127.0.0.1:5432/zhblogs_test',
  VALKEY_URL: 'redis://127.0.0.1:6379',
  API_WEB_BASE_URL: 'http://127.0.0.1:9101',
  WEB_PUBLIC_BASE_URL: 'http://127.0.0.1:9101',
  API_CORS_ORIGINS: 'http://127.0.0.1:9101,http://localhost:9101',
  API_GITHUB_CLIENT_ID: 'github-client-id-for-tests',
  API_GITHUB_CLIENT_SECRET: 'github-client-secret-for-tests',
  API_GITHUB_CALLBACK_URL: 'http://127.0.0.1:9101/auth/github/callback',
  API_GITHUB_SCOPE: 'read:user,user:email',
  API_JWT_ACCESS_SECRET: 'zhblogs-api-access-secret-for-dev-and-test-only',
  API_JWT_REFRESH_SECRET: 'zhblogs-api-refresh-secret-for-dev-and-test-only',
  API_JWT_ACCESS_TTL_SECONDS: 21600,
  API_JWT_REFRESH_TTL_SECONDS: 604800,
  API_SMTP_HOST: 'smtp.test.local',
  API_SMTP_PORT: 587,
  API_SMTP_SECURE: false,
  API_SMTP_USER: 'smtp-user',
  API_SMTP_PASS: 'smtp-pass',
  API_SMTP_FROM: 'noreply@test.local',
  API_INTERNAL_TOKEN: 'zhblogs-internal-token-for-tests',
} as const;

export const TEST_AUTH_COOKIES = {
  access: 'zhblogs_access_token',
  refresh: 'zhblogs_refresh_token',
} as const;

export const TEST_HEADERS = {
  internalToken: 'x-internal-token',
} as const;
