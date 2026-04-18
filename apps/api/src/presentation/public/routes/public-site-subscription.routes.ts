import type { FastifyInstance } from 'fastify';

import { loadPublicSiteSubscriptions } from '@/application/public/usecase/public-site-subscription.usecase';
import { publicSiteSubscriptionResponseSchema } from '@/presentation/public/dto/public-site-subscription-response.dto';

import { toPositiveInt } from './public-route.helpers';

export const registerPublicSiteSubscriptionRoutes = (app: FastifyInstance) => {
  app.get(
    '/api/public/subscriptions',
    {
      schema: {
        tags: ['public'],
        summary: 'Public subscription article archive',
        response: {
          200: publicSiteSubscriptionResponseSchema,
        },
      },
      config: {
        rateLimit: {
          max: 120,
          timeWindow: '1 minute',
        },
      },
    },
    async (request) => {
      const query = request.query as Record<string, unknown>;

      return {
        ok: true,
        data: await loadPublicSiteSubscriptions(
          app,
          toPositiveInt(query.page, 1),
          toPositiveInt(query.pageSize, 20),
        ),
      };
    },
  );
};
