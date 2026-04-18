import {
  pagedResponseSchema,
  publicSiteArticleItemSchema,
  publicSiteCheckItemSchema,
  publicSiteItemSchema,
} from './public-site-shared.dto';

export const publicSitesResponseSchema = {
  type: 'object',
  properties: {
    ok: { type: 'boolean' },
    data: {
      type: 'object',
      properties: {
        items: {
          type: 'array',
          items: publicSiteItemSchema,
        },
      },
      required: ['items'],
    },
  },
  required: ['ok', 'data'],
} as const;

export const directoryMetaResponseSchema = {
  type: 'object',
  properties: {
    ok: { type: 'boolean' },
    data: {
      type: 'object',
      properties: {
        stats: {
          type: 'object',
          properties: {
            totalSites: { type: 'number' },
            normalSites: { type: 'number' },
            abnormalSites: { type: 'number' },
            rssSites: { type: 'number' },
          },
          required: ['totalSites', 'normalSites', 'abnormalSites', 'rssSites'],
        },
        filters: {
          type: 'object',
          properties: {
            mainTags: {
              type: 'array',
              items: {
                type: 'object',
                properties: { id: { type: 'string' }, name: { type: 'string' } },
                required: ['id', 'name'],
              },
            },
            subTags: {
              type: 'array',
              items: {
                type: 'object',
                properties: { id: { type: 'string' }, name: { type: 'string' } },
                required: ['id', 'name'],
              },
            },
            warningTags: {
              type: 'array',
              items: {
                type: 'object',
                properties: {
                  id: { type: 'string' },
                  machineKey: { type: ['string', 'null'] },
                  name: { type: 'string' },
                },
                required: ['id', 'machineKey', 'name'],
              },
            },
            programs: {
              type: 'array',
              items: {
                type: 'object',
                properties: {
                  id: { type: 'string' },
                  name: { type: 'string' },
                },
                required: ['id', 'name'],
              },
            },
          },
          required: ['mainTags', 'subTags', 'warningTags', 'programs'],
        },
        defaults: {
          type: 'object',
          properties: {
            pageSize: { type: 'number' },
            random: { type: 'boolean' },
            statusMode: { type: 'string' },
          },
          required: ['pageSize', 'random', 'statusMode'],
        },
      },
      required: ['stats', 'filters', 'defaults'],
    },
  },
  required: ['ok', 'data'],
} as const;

export const directoryResponseSchema = {
  type: 'object',
  properties: {
    ok: { type: 'boolean' },
    data: {
      type: 'object',
      properties: {
        items: { type: 'array', items: publicSiteItemSchema },
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
        query: {
          type: 'object',
          properties: {
            q: { type: 'string' },
            main: { type: 'array', items: { type: 'string' } },
            sub: { type: 'array', items: { type: 'string' } },
            warning: { type: 'array', items: { type: 'string' } },
            program: { type: 'array', items: { type: 'string' } },
            statusMode: { type: 'string' },
            random: { type: 'boolean' },
            sort: { type: ['string', 'null'] },
            order: { type: 'string' },
            randomSeed: { type: 'string' },
          },
          required: [
            'q',
            'main',
            'sub',
            'warning',
            'program',
            'statusMode',
            'random',
            'sort',
            'order',
            'randomSeed',
          ],
        },
      },
      required: ['items', 'pagination', 'query'],
    },
  },
  required: ['ok', 'data'],
} as const;

export const publicSiteRandomResponseSchema = {
  type: 'object',
  properties: {
    ok: { type: 'boolean' },
    data: {
      type: 'object',
      properties: {
        site: {
          type: ['object', 'null'],
          properties: publicSiteItemSchema.properties,
          required: publicSiteItemSchema.required,
        },
        availableTypes: {
          type: 'array',
          items: { type: 'string' },
        },
        filters: {
          type: 'object',
          properties: {
            recommend: { type: 'boolean' },
            type: { type: 'string' },
          },
          required: ['recommend', 'type'],
        },
        failureReason: {
          type: ['string', 'null'],
          enum: [
            'UNKNOWN_PARAM',
            'INVALID_PARAMS',
            'INVALID_RECOMMEND',
            'INVALID_TYPE',
            'DUPLICATE_PARAM',
            'NO_MATCH',
            null,
          ],
        },
      },
      required: ['site', 'availableTypes', 'filters', 'failureReason'],
    },
  },
  required: ['ok', 'data'],
} as const;

export const publicSiteDetailResponseSchema = {
  type: 'object',
  properties: {
    ok: { type: 'boolean' },
    data: {
      type: ['object', 'null'],
      properties: {
        ...publicSiteItemSchema.properties,
        reason: { type: ['string', 'null'] },
        feeds: {
          type: 'array',
          items: {
            type: 'object',
            properties: {
              name: { type: ['string', 'null'] },
              url: { type: 'string' },
              type: { type: ['string', 'null'] },
              isDefault: { type: 'boolean' },
            },
            required: ['name', 'url', 'type', 'isDefault'],
          },
        },
        architecture: {
          type: 'object',
          properties: {
            program: {
              type: ['object', 'null'],
              properties: {
                id: { type: 'string' },
                name: { type: 'string' },
                isOpenSource: { type: 'boolean' },
                websiteUrl: { type: ['string', 'null'] },
                repoUrl: { type: ['string', 'null'] },
              },
              required: ['id', 'name', 'isOpenSource', 'websiteUrl', 'repoUrl'],
            },
          },
          required: ['program'],
        },
        heartbeatChecks: {
          type: 'array',
          items: publicSiteCheckItemSchema,
        },
      },
      required: [
        ...publicSiteItemSchema.required,
        'reason',
        'feeds',
        'architecture',
        'heartbeatChecks',
      ],
    },
  },
  required: ['ok', 'data'],
} as const;

export const publicSiteArticleResponseSchema = pagedResponseSchema(publicSiteArticleItemSchema);

export const publicSiteCheckResponseSchema = pagedResponseSchema(publicSiteCheckItemSchema);

export const publicSiteFeedbackResponseSchema = {
  type: 'object',
  properties: {
    ok: { type: 'boolean' },
    data: {
      type: 'object',
      properties: {
        id: { type: 'string' },
      },
      required: ['id'],
    },
  },
  required: ['ok', 'data'],
} as const;

export const publicSiteAccessResponseSchema = {
  type: 'object',
  properties: {
    ok: { type: 'boolean' },
    data: {
      type: 'object',
      properties: {
        recorded: { type: 'boolean' },
      },
      required: ['recorded'],
    },
  },
  required: ['ok', 'data'],
} as const;

export const siteDirectoryPreferenceResponseSchema = {
  type: 'object',
  properties: {
    ok: { type: 'boolean' },
    data: {
      type: 'object',
      properties: {
        randomMode: { type: 'string' },
        randomSeed: { type: ['string', 'null'] },
      },
      required: ['randomMode', 'randomSeed'],
    },
  },
  required: ['ok', 'data'],
} as const;
