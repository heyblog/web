import { siteConfig } from '../site.config.ts';

import { crawlerDisallowPatterns } from './indexing.ts';

export function robotsText(): string {
  const exclusions = crawlerDisallowPatterns.map((path) => `Disallow: ${path}`).join('\n');
  const discovery = ['*', 'OAI-SearchBot', 'Claude-SearchBot']
    .map((agent) => `User-agent: ${agent}\nAllow: /\n${exclusions}`)
    .join('\n\n');
  const training = ['GPTBot', 'ClaudeBot', 'Google-Extended', 'CCBot']
    .map((agent) => `User-agent: ${agent}\nDisallow: /`)
    .join('\n\n');
  return `${discovery}\n\n${training}\n\nSitemap: ${siteConfig.url}/sitemap.xml\n`;
}

export function llmsText(): string {
  return `# HeyBlog\n\n> ${siteConfig.description}。\n\n本站整理公开博客资料、订阅入口与友链关系。博客资料页是目录介绍，第三方博客及其文章的权利归原作者所有。\n\n## 公开内容\n\n- [博客目录](${siteConfig.url}/site)：收录博客的公开资料与原站链接。\n- [项目博客](${siteConfig.url}/blog)：HeyBlog 项目文章和建设记录，随版本构建发布。\n- [公告](${siteConfig.url}/announcements)：公开项目公告。\n- [项目文档](${siteConfig.url}/docs)：使用和协作说明。\n- [项目成员](${siteConfig.url}/members)：公开成员和贡献者信息。\n- [Sitemap](${siteConfig.url}/sitemap.xml)：本站公开页面的完整发现入口。\n\n## 使用边界\n\n- [数据使用声明](${siteConfig.url}/data-use)\n- [版权声明](${siteConfig.url}/copyright)\n- [爬虫协议](${siteConfig.url}/crawler)：HeyBlogBot 对外访问的规则。\n- [robots.txt](${siteConfig.url}/robots.txt)：访问本站的自动抓取偏好。\n\n本文件提供内容导航，不授予批量复制、转载或模型训练许可；具体内容遵循原作者与本站声明。\n`;
}
