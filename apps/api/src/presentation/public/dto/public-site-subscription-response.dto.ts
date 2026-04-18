import { publicSiteArticleItemSchema } from './public-site-shared.dto';

const subscriptionSummarySchema = {
  type: 'object',
  properties: {
    todayArticles: { type: 'number' },
    weekArticles: { type: 'number' },
    totalArticles: { type: 'number' },
    siteCount: { type: 'number' },
  },
  required: ['todayArticles', 'weekArticles', 'totalArticles', 'siteCount'],
} as const;

const subscriptionItemSchema = {
  type: 'object',
  properties: {
    ...publicSiteArticleItemSchema.properties,
    site: {
      type: 'object',
      properties: {
        id: { type: 'string' },
        slug: { type: 'string' },
        name: { type: 'string' },
        url: { type: 'string' },
      },
      required: ['id', 'slug', 'name', 'url'],
    },
  },
  required: [...publicSiteArticleItemSchema.required, 'site'],
} as const;

export const publicSiteSubscriptionResponseSchema = {
  type: 'object',
  properties: {
    ok: { type: 'boolean' },
    data: {
      type: 'object',
      properties: {
        summary: subscriptionSummarySchema,
        items: {
          type: 'array',
          items: subscriptionItemSchema,
        },
        pagination: {
          type: 'object',
          properties: {
            page: { type: 'number' },
            pageSize: { type: 'number' },
            totalItems: { type: 'number' },
            totalPages: { type: 'number' },
          },
          required: ['page', 'pageSize', 'totalItems', 'totalPages'],
        },
      },
      required: ['summary', 'items', 'pagination'],
    },
  },
  required: ['ok', 'data'],
} as const;
