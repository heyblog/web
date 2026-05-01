import fastify from 'fastify';

import { cachePlugin } from '@/infrastructure/app/cache/app-cache.service';
import { drizzlePlugin } from '@/infrastructure/app/db/app-database.service';
import {
  type AppBootstrapOptions,
  buildAppConfigData,
  configPlugin,
} from '@/infrastructure/app/http/app-config.service';
import { securityPlugin } from '@/infrastructure/app/http/app-security.service';
import { authPlugin } from '@/infrastructure/auth/http/auth.service';
import { registerAuthRoutes } from '@/presentation/auth/routes/auth.controller';
import { registerHealthRoutes } from '@/presentation/health/routes/health.controller';
import { registerInternalJobRoutes } from '@/presentation/jobs/routes';
import { registerManagementRoutes } from '@/presentation/management/routes/management.controller';
import { registerManagementAnnouncementRoutes } from '@/presentation/management/routes/management-announcement.controller';
import { registerPresenceRoutes } from '@/presentation/presence/routes/presence.controller';
import { registerPublicRoutes } from '@/presentation/public/routes/public.controller';
import { registerInternalSiteCorrectionRoute } from '@/presentation/sites/routes/internal-site-correction.controller';
import { registerSiteRoutes } from '@/presentation/sites/routes/site.controller';
import { registerAdminUserRoutes } from '@/presentation/user/routes/admin-user.controller';
import { registerSiteDirectoryPreferenceRoutes } from '@/presentation/user/routes/site-directory.controller';
import { getLoggerOptions } from '@/shared/runtime/service/app-logger.service';

export function createApp(options: AppBootstrapOptions = {}) {
  const configData = buildAppConfigData(options.envOverrides);
  const app = fastify({
    logger: configData.NODE_ENV === 'test' ? false : getLoggerOptions(configData),
  });

  app.register(configPlugin, options);
  app.register(securityPlugin);
  app.register(drizzlePlugin);
  app.register(cachePlugin);
  app.register(authPlugin);

  app.after(() => {
    registerHealthRoutes(app);
    registerPresenceRoutes(app);
    registerAuthRoutes(app);
    registerAdminUserRoutes(app);
    registerManagementAnnouncementRoutes(app);
    registerManagementRoutes(app);
    registerSiteDirectoryPreferenceRoutes(app);
    registerInternalJobRoutes(app);
    registerInternalSiteCorrectionRoute(app);
    registerPublicRoutes(app);
    registerSiteRoutes(app);
  });

  return app;
}
