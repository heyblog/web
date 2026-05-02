import fastifyEnv from '@fastify/env';
import fp from 'fastify-plugin';

export interface AppConfig {
  NODE_ENV: 'development' | 'test' | 'production';
  DATABASE_URL: string;
  VALKEY_URL: string;
  API_PORT?: number;
  API_LOG_LEVEL?: string;
  API_LOG_DIR?: string;
  API_WEB_BASE_URL: string;
  API_CORS_ORIGINS: string;
  API_GITHUB_CLIENT_ID: string;
  API_GITHUB_CLIENT_SECRET: string;
  API_GITHUB_CALLBACK_URL: string;
  API_GITHUB_SCOPE: string;
  API_COOKIE_DOMAIN?: string;
  API_JWT_ACCESS_SECRET: string;
  API_JWT_REFRESH_SECRET: string;
  API_JWT_ACCESS_TTL_SECONDS: number;
  API_JWT_REFRESH_TTL_SECONDS: number;
  API_SMTP_HOST: string;
  API_SMTP_PORT: number;
  API_SMTP_SECURE: boolean;
  API_SMTP_USER: string;
  API_SMTP_PASS: string;
  API_SMTP_FROM: string;
  API_INTERNAL_TOKEN: string;
}

export interface AppBootstrapOptions {
  envOverrides?: Partial<AppConfig>;
  disableExternalServices?: boolean;
}

export type AppConfigData = NodeJS.ProcessEnv & Partial<AppConfig>;

export const buildAppConfigData = (
  envOverrides: AppBootstrapOptions['envOverrides'] = {},
): AppConfigData =>
  ({
    ...process.env,
    ...envOverrides,
  }) as AppConfigData;

declare module 'fastify' {
  interface FastifyInstance {
    config: AppConfig;
    bootstrapOptions: AppBootstrapOptions;
  }
}

const envSchema = {
  type: 'object',
  required: [
    'NODE_ENV',
    'DATABASE_URL',
    'VALKEY_URL',
    'API_GITHUB_CLIENT_ID',
    'API_GITHUB_CLIENT_SECRET',
    'API_GITHUB_CALLBACK_URL',
    'API_GITHUB_SCOPE',
    'API_SMTP_HOST',
    'API_SMTP_PORT',
    'API_SMTP_SECURE',
    'API_SMTP_USER',
    'API_SMTP_PASS',
    'API_SMTP_FROM',
  ],
  properties: {
    NODE_ENV: {
      type: 'string',
      enum: ['development', 'test', 'production'],
    },
    API_PORT: {
      type: 'number',
      default: 9201,
    },
    DATABASE_URL: {
      type: 'string',
      minLength: 1,
    },
    VALKEY_URL: {
      type: 'string',
      minLength: 1,
    },
    API_LOG_LEVEL: {
      type: 'string',
    },
    API_LOG_DIR: {
      type: 'string',
    },
    API_WEB_BASE_URL: {
      type: 'string',
      default: 'http://127.0.0.1:9101',
    },
    API_CORS_ORIGINS: {
      type: 'string',
      default: 'http://127.0.0.1:9101,http://localhost:9101',
    },
    API_GITHUB_CLIENT_ID: {
      type: 'string',
      minLength: 1,
    },
    API_GITHUB_CLIENT_SECRET: {
      type: 'string',
      minLength: 1,
    },
    API_GITHUB_CALLBACK_URL: {
      type: 'string',
      minLength: 1,
    },
    API_GITHUB_SCOPE: {
      type: 'string',
      minLength: 1,
    },
    API_COOKIE_DOMAIN: {
      type: 'string',
    },
    API_JWT_ACCESS_SECRET: {
      type: 'string',
      default: 'zhblogs-api-access-secret-for-dev-and-test-only',
      minLength: 16,
    },
    API_JWT_REFRESH_SECRET: {
      type: 'string',
      default: 'zhblogs-api-refresh-secret-for-dev-and-test-only',
      minLength: 16,
    },
    API_JWT_ACCESS_TTL_SECONDS: {
      type: 'number',
      default: 21600, // 6 hours
    },
    API_JWT_REFRESH_TTL_SECONDS: {
      type: 'number',
      default: 604800, // 1 week
    },
    API_SMTP_HOST: {
      type: 'string',
      minLength: 1,
    },
    API_SMTP_PORT: {
      type: 'number',
    },
    API_SMTP_SECURE: {
      type: 'boolean',
    },
    API_SMTP_USER: {
      type: 'string',
      minLength: 1,
    },
    API_SMTP_PASS: {
      type: 'string',
      minLength: 1,
    },
    API_SMTP_FROM: {
      type: 'string',
      minLength: 1,
    },
    API_INTERNAL_TOKEN: {
      type: 'string',
      default: 'zhblogs-internal-dev-token',
      minLength: 16,
    },
  },
} as const;

const cookieDomainPattern =
  /^(localhost|\d{1,3}(?:\.\d{1,3}){3}|[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*)$/i;

const validateCookieDomain = (value: string | undefined): void => {
  if (!value) {
    return;
  }

  if (value.includes('://') || value.includes('/') || value.includes(':')) {
    throw new Error('API_COOKIE_DOMAIN must be a bare hostname or left empty');
  }

  if (!cookieDomainPattern.test(value)) {
    throw new Error('API_COOKIE_DOMAIN must be a valid hostname');
  }
};

export const configPlugin = fp<AppBootstrapOptions>(
  async (app, options) => {
    app.decorate('bootstrapOptions', options);

    await app.register(fastifyEnv, {
      confKey: 'config',
      schema: envSchema,
      data: buildAppConfigData(options.envOverrides),
      dotenv: false,
    });

    validateCookieDomain(app.config.API_COOKIE_DOMAIN);
  },
  { name: 'config' },
);
