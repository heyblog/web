import { file, glob } from 'astro/loaders';
import { z } from 'astro/zod';
import { defineCollection, reference } from 'astro:content';

import { contentSources } from './content-sources.mjs';

export enum MemberStatus {
  ACTIVE = '在职',
  INACTIVE = '暂离',
  ALUMNI = '离开',
}

export const MemberStatusKeys = Object.keys(MemberStatus) as (keyof typeof MemberStatus)[];

const members = defineCollection({
  loader: file(`./contents/${contentSources.members.path}`),
  schema: z.object({
    // 个人信息
    id: z.string(), // Github 用户名，即个人主页中的 URL 中的最后一段
    nickname: z.string().optional(), // 昵称，为空时使用 id 作为昵称
    url: z.url().optional(), // 个人主页的 URL，为空时使用 Github 主页作为个人主页

    title: z.string(), // 贡献职责称谓，为空时使用默认值
    description: z.string(), // 贡献边界说明
    tags: z.array(z.string()).default([]), // 贡献边界标签
    status: z.enum(MemberStatusKeys).default('ACTIVE'), //贡献状态
    join_time: z.iso.date().optional(), // 加入时间
    leave_time: z.iso.date().optional(), // 离开时间
    sort: z.number().int().optional(), // 排序值，越小越靠前，为空时使用 id 排序
  }),
});

const blogs = defineCollection({
  loader: glob({
    pattern: contentSources.blogs.pattern,
    base: `./contents/${contentSources.blogs.path}`,
  }),
  schema: z.object({
    title: z.string(),
    description: z.string(),
    create_time: z.date(),
    category: z.string(),
    editors: z.array(reference('members')),
    top: z.boolean().default(false),
  }),
});

const docs = defineCollection({
  loader: glob({
    pattern: contentSources.docs.pattern,
    base: `./contents/${contentSources.docs.path}`,
  }),
  schema: z.object({
    title: z.string(),
    description: z.string(),
    create_time: z.date(),
    editors: z.array(reference('members')),
  }),
});

const pages = defineCollection({
  loader: glob({
    pattern: contentSources.pages.pattern,
    base: `./contents/${contentSources.pages.path}`,
  }),
  schema: z.object({
    title: z.string(),
    create_time: z.date().optional(),
    update_time: z.date().optional(),
  }),
});

export const collections = {
  blogs,
  docs,
  pages,
  members,
};
