import type { SiteConfig } from './site-config.types';

export const siteConfig: SiteConfig = {
  name: 'HeyBlog',
  title: 'HeyBlog',
  description: '尝试发现、整理公开的个人博客站点',
  url: 'https://www.heyblog.net',
  language: 'zh-CN',
  locale: 'zh_CN',
  robots: 'index, follow',
  themeStorageKey: 'heyblog-theme',
  openGraph: {
    type: 'website',
    imagePath: '/og-default.svg',
    imageAlt: 'HeyBlog 分享卡片',
  },
  twitterCard: 'summary_large_image',
  navigation: {
    primary: [
      { label: '博客列表', href: '/site', match: 'prefix', sort: 10 },
      { label: '项目动态', href: '/blog', match: 'prefix', sort: 20 },
      { label: '成员', href: '/members', match: 'prefix', sort: 30 },
    ],
    submission: {
      primary: {
        label: '提交博客',
        href: '/site/submissions/new',
        match: 'exact',
        sort: 10,
      },
      menu: [
        { label: '更新收录信息', href: '/site/submissions/update', match: 'exact', sort: 10 },
        { label: '移除收录', href: '/site/submissions/delete', match: 'exact', sort: 20 },
        { label: '恢复收录', href: '/site/submissions/restore', match: 'exact', sort: 30 },
        { label: '查询提交进度', href: '/site/submissions/query', match: 'exact', sort: 40 },
      ],
    },
  },
  footer: {
    copyrightStartYear: 2022,
    copyrightOwner: 'HeyBlog',
    registration: {
      label: '陇ICP备 2021003047号-6',
      href: 'https://beian.miit.gov.cn/',
      external: true,
    },
    statementLinks: [
      { label: '免责声明', href: '/disclaimer' },
      { label: '版权声明', href: '/copyright' },
      { label: '隐私说明', href: '/privacy' },
      { label: '社区公约', href: '/community' },
    ],
    friendLinks: [
      { label: '开往', href: 'https://www.travellings.cn/', external: true },
      { label: '博友圈', href: 'https://www.boyouquan.com/', external: true },
      { label: 'BlogsClub', href: 'https://www.blogsclub.org/', external: true },
      { label: 'BlogFinder', href: 'https://bf.zzxworld.com/', external: true },
      {
        label: '中文独立博客列表',
        href: 'https://github.com/timqian/chinese-independent-blogs',
        external: true,
      },
      { label: 'Ooh.Directory', href: 'https://ooh.directory/', external: true },
    ],
  },
};
