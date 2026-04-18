export const warningTagSchema = {
  type: 'object',
  properties: {
    machineKey: { type: 'string' },
    name: { type: 'string' },
    description: { type: ['string', 'null'] },
  },
  required: ['machineKey', 'name', 'description'],
} as const;

export const publicSiteItemSchema = {
  type: 'object',
  properties: {
    id: { type: 'string' },
    bid: { type: ['string', 'null'] },
    slug: { type: 'string' },
    name: { type: 'string' },
    url: { type: 'string' },
    sign: { type: 'string' },
    feedUrl: { type: ['string', 'null'] },
    sitemap: { type: ['string', 'null'] },
    linkPage: { type: ['string', 'null'] },
    featured: { type: 'boolean' },
    status: { type: 'string' },
    accessScope: { type: 'string' },
    joinTime: { type: 'string' },
    updateTime: { type: 'string' },
    latestPublishedTime: { type: ['string', 'null'] },
    articleCount: { type: 'number' },
    visitCount: { type: 'number' },
    primaryTag: { type: ['string', 'null'] },
    subTags: {
      type: 'array',
      items: { type: 'string' },
    },
    warningTags: {
      type: 'array',
      items: warningTagSchema,
    },
  },
  required: [
    'id',
    'bid',
    'slug',
    'name',
    'url',
    'sign',
    'feedUrl',
    'sitemap',
    'linkPage',
    'featured',
    'status',
    'accessScope',
    'joinTime',
    'updateTime',
    'latestPublishedTime',
    'articleCount',
    'visitCount',
    'primaryTag',
    'subTags',
    'warningTags',
  ],
} as const;

export const publicSiteArticleItemSchema = {
  type: 'object',
  properties: {
    id: { type: 'string' },
    title: { type: 'string' },
    articleUrl: { type: 'string' },
    summary: { type: ['string', 'null'] },
    publishedTime: { type: ['string', 'null'] },
    fetchedTime: { type: 'string' },
    source: {
      type: 'object',
      properties: {
        feedName: { type: ['string', 'null'] },
        feedUrl: { type: ['string', 'null'] },
        feedType: { type: ['string', 'null'] },
      },
      required: ['feedName', 'feedUrl', 'feedType'],
    },
  },
  required: ['id', 'title', 'articleUrl', 'summary', 'publishedTime', 'fetchedTime', 'source'],
} as const;

export const publicSiteCheckItemSchema = {
  type: 'object',
  properties: {
    id: { type: 'string' },
    region: { type: 'string' },
    result: { type: 'string' },
    statusCode: { type: ['number', 'null'] },
    responseTimeMs: { type: ['number', 'null'] },
    durationMs: { type: ['number', 'null'] },
    message: { type: ['string', 'null'] },
    finalUrl: { type: ['string', 'null'] },
    contentVerified: { type: 'boolean' },
    checkTime: { type: 'string' },
  },
  required: [
    'id',
    'region',
    'result',
    'statusCode',
    'responseTimeMs',
    'durationMs',
    'message',
    'finalUrl',
    'contentVerified',
    'checkTime',
  ],
} as const;

export const pagedResponseSchema = (itemSchema: unknown) =>
  ({
    type: 'object',
    properties: {
      ok: { type: 'boolean' },
      data: {
        type: ['object', 'null'],
        properties: {
          items: {
            type: 'array',
            items: itemSchema,
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
        required: ['items', 'pagination'],
      },
    },
    required: ['ok', 'data'],
  }) as const;
