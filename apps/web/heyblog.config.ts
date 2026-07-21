export interface SiteConfig {
  name: string;
  title: string;
  description: string;
  url: string;
  language: string;
  locale: string;
  robots: string;
  openGraph: {
    type: 'website' | 'article';
    imagePath: string;
    imageAlt: string;
  };
  twitterCard: 'summary' | 'summary_large_image';
}

export const siteConfig = {
  name: 'HeyBlog',
  title: 'HeyBlog',
  description: '收集并链接所有的个人博客站点',
  url: 'https://www.heyblog.net',
  language: 'zh-CN',
  locale: 'zh_CN',
  robots: 'index, follow',
  openGraph: {
    type: 'website',
    imagePath: '/og-default.svg',
    imageAlt: 'HeyBlog 分享卡片',
  },
  twitterCard: 'summary_large_image',
} as const satisfies SiteConfig;
