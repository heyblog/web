import type { FastifyInstance, FastifyReply, FastifyRequest } from 'fastify';

import {
  loadPublicSiteArticles,
  loadPublicSiteDetail,
} from '@/application/public/usecase/public-site.detail.usecase';
import { submitPublicSiteFeedback } from '@/application/public/usecase/public-site.preference.usecase';
import { loadPublicSiteChecks } from '@/application/public/usecase/public-site-checks.usecase';
import {
  publicSiteArticleResponseSchema,
  publicSiteCheckResponseSchema,
  publicSiteDetailResponseSchema,
  publicSiteFeedbackResponseSchema,
} from '@/presentation/public/dto/public-response.dto';

import { toPositiveInt } from './public-route.helpers';

const normalizeFeedbackBody = (body: Record<string, unknown> | null) => {
  if (
    !body ||
    typeof body.reasonType !== 'string' ||
    typeof body.feedbackContent !== 'string' ||
    !body.feedbackContent.trim()
  ) {
    return null;
  }

  return {
    reasonType: body.reasonType as never,
    feedbackContent: body.feedbackContent,
    reporterName: typeof body.reporterName === 'string' ? body.reporterName : null,
    reporterEmail: typeof body.reporterEmail === 'string' ? body.reporterEmail : null,
    notifyByEmail: body.notifyByEmail === true,
  };
};

type FeedbackRequest = FastifyRequest<{ Params: { slug: string } }>;

const registerSiteDetailRoute = (app: FastifyInstance) => {
  app.get<{ Params: { slug: string } }>(
    '/api/public/sites/:slug',
    {
      schema: {
        response: {
          200: publicSiteDetailResponseSchema,
        },
      },
      config: {
        rateLimit: {
          max: 120,
          timeWindow: '1 minute',
        },
      },
    },
    async (request, reply) => {
      const detail = await loadPublicSiteDetail(app, request.params.slug);

      if (!detail) {
        reply.code(404);
      }

      return {
        ok: Boolean(detail),
        data: detail,
      };
    },
  );
};

const registerSiteArticlesRoute = (app: FastifyInstance) => {
  app.get<{ Params: { slug: string } }>(
    '/api/public/sites/:slug/articles',
    {
      schema: {
        response: {
          200: publicSiteArticleResponseSchema,
        },
      },
      config: {
        rateLimit: {
          max: 120,
          timeWindow: '1 minute',
        },
      },
    },
    async (request, reply) => {
      const query = request.query as Record<string, unknown>;
      const data = await loadPublicSiteArticles(
        app,
        request.params.slug,
        toPositiveInt(query.page, 1),
        toPositiveInt(query.pageSize, 20),
      );

      if (!data) {
        reply.code(404);
      }

      return {
        ok: Boolean(data),
        data,
      };
    },
  );
};

const registerSiteChecksRoute = (app: FastifyInstance) => {
  app.get<{ Params: { slug: string } }>(
    '/api/public/sites/:slug/checks',
    {
      schema: {
        response: {
          200: publicSiteCheckResponseSchema,
        },
      },
      config: {
        rateLimit: {
          max: 120,
          timeWindow: '1 minute',
        },
      },
    },
    async (request, reply) => {
      const query = request.query as Record<string, unknown>;
      const data = await loadPublicSiteChecks(
        app,
        request.params.slug,
        toPositiveInt(query.page, 1),
        toPositiveInt(query.pageSize, 20),
      );

      if (!data) {
        reply.code(404);
      }

      return {
        ok: Boolean(data),
        data,
      };
    },
  );
};

const siteFeedbackRouteConfig = {
  schema: {
    response: {
      200: publicSiteFeedbackResponseSchema,
    },
  },
  config: {
    rateLimit: {
      max: 20,
      timeWindow: '10 minutes',
    },
  },
};

const buildFeedbackHandler =
  (app: FastifyInstance) => async (request: FeedbackRequest, reply: FastifyReply) => {
    const body = normalizeFeedbackBody(request.body as Record<string, unknown> | null);

    if (!body) {
      reply.code(400);
      return {
        ok: false,
        data: null,
      };
    }

    try {
      const data = await submitPublicSiteFeedback(app, {
        slug: request.params.slug,
        ...body,
      });

      return {
        ok: true,
        data,
      };
    } catch (error) {
      if (error instanceof Error && error.message === 'site_not_found') {
        reply.code(404);
        return {
          ok: false,
          data: null,
        };
      }

      app.log.error({ error }, 'failed to submit public site feedback');
      reply.code(500);
      return {
        ok: false,
        data: null,
      };
    }
  };

const registerSiteFeedbackRoute = (app: FastifyInstance) => {
  app.post<{ Params: { slug: string } }>(
    '/api/public/sites/:slug/feedback',
    siteFeedbackRouteConfig,
    buildFeedbackHandler(app),
  );
};

export const registerPublicSiteDetailRoutes = (app: FastifyInstance) => {
  registerSiteDetailRoute(app);
  registerSiteArticlesRoute(app);
  registerSiteChecksRoute(app);
  registerSiteFeedbackRoute(app);
};
