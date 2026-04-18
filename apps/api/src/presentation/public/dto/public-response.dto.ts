import {
  directoryMetaResponseSchema,
  directoryResponseSchema,
  publicSiteAccessResponseSchema,
  publicSiteArticleResponseSchema,
  publicSiteCheckResponseSchema,
  publicSiteDetailResponseSchema,
  publicSiteFeedbackResponseSchema,
  publicSiteRandomResponseSchema,
  publicSitesResponseSchema,
  siteDirectoryPreferenceResponseSchema,
} from '@/presentation/public/dto/public-site-response.dto';
import { publicSiteSubscriptionResponseSchema as subscriptionResponseSchema } from '@/presentation/public/dto/public-site-subscription-response.dto';

const homeSummarySchema = {
  type: 'object',
  properties: {
    totalSites: { type: 'number' },
    featuredSites: { type: 'number' },
    todayUpdates: { type: 'number' },
  },
  required: ['totalSites', 'featuredSites', 'todayUpdates'],
} as const;

const homeResponseSchema = {
  type: 'object',
  properties: {
    ok: { type: 'boolean' },
    data: {
      type: 'object',
      properties: {
        summary: homeSummarySchema,
      },
      required: ['summary'],
    },
  },
  required: ['ok', 'data'],
} as const;

const announcementItemSchema = {
  type: 'object',
  properties: {
    id: { type: 'string' },
    title: { type: 'string' },
    content: { type: ['string', 'null'] },
    publishTime: { type: ['string', 'null'] },
  },
  required: ['id', 'title', 'content', 'publishTime'],
} as const;

const currentAnnouncementResponseSchema = {
  type: 'object',
  properties: {
    ok: { type: 'boolean' },
    data: {
      type: ['object', 'null'],
      properties: {
        ...announcementItemSchema.properties,
      },
      required: announcementItemSchema.required,
    },
  },
  required: ['ok', 'data'],
} as const;

const announcementArchiveItemSchema = {
  type: 'object',
  properties: {
    id: { type: 'string' },
    title: { type: 'string' },
    content: { type: ['string', 'null'] },
    status: { type: 'string' },
    publishTime: { type: ['string', 'null'] },
    expireTime: { type: ['string', 'null'] },
  },
  required: ['id', 'title', 'content', 'status', 'publishTime', 'expireTime'],
} as const;

const announcementsResponseSchema = {
  type: 'object',
  properties: {
    ok: { type: 'boolean' },
    data: {
      type: 'object',
      properties: {
        items: {
          type: 'array',
          items: announcementArchiveItemSchema,
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
} as const;

export {
  announcementsResponseSchema,
  currentAnnouncementResponseSchema,
  directoryMetaResponseSchema,
  directoryResponseSchema,
  homeResponseSchema,
  publicSiteAccessResponseSchema,
  publicSiteArticleResponseSchema,
  publicSiteCheckResponseSchema,
  publicSiteDetailResponseSchema,
  publicSiteFeedbackResponseSchema,
  publicSiteRandomResponseSchema,
  publicSitesResponseSchema,
  subscriptionResponseSchema as publicSiteSubscriptionResponseSchema,
  siteDirectoryPreferenceResponseSchema,
};
