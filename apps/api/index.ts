import { createApp } from '@/app/api/service/api.service';

const app = createApp();

const start = async () => {
  try {
    await app.ready();

    const port = app.config.API_PORT ?? 9201;

    app.log.info({ port, env: app.config.NODE_ENV }, 'api server starting');

    await app.listen({ host: '0.0.0.0', port });
  } catch (error) {
    app.log.error(error);
    process.exit(1);
  }
};

void start();
