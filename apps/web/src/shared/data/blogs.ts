export type BlogCardTone = 'amber' | 'blue' | 'emerald' | 'red' | 'stone';
export type BlogCardUpdatedTone = 'amber' | 'blue' | 'emerald' | 'stone';

export interface BlogCardEntry {
  slug: string;
  name: string;
  domain: string;
  href: string;
  shortCode: string;
  primaryTag: string;
  summary: string;
  profile: string;
  highlights: string[];
  subTags: string[];
  warningTags: Array<{
    machineKey: string;
    name: string;
    description: string | null;
  }>;
  joinedAt: string;
  joinedLabel: string;
  updatedLabel: string | null;
  updatedTone: BlogCardUpdatedTone | null;
  articleCount?: string;
  visitCount: string;
  tone: BlogCardTone;
  rssUrl?: string;
  sitemapUrl?: string;
  featured?: boolean;
}

const normalizeSubTags = (primaryTag: string, subTags: string[]): string[] =>
  Array.from(
    new Set(subTags.map((tag) => tag.trim()).filter((tag) => tag && tag !== primaryTag)),
  ).slice(0, 10);

const createBlogEntry = (
  entry: Omit<BlogCardEntry, 'subTags' | 'warningTags'> & {
    subTags: string[];
    warningTags?: BlogCardEntry['warningTags'];
  },
): BlogCardEntry => ({
  ...entry,
  subTags: normalizeSubTags(entry.primaryTag, entry.subTags),
  warningTags: entry.warningTags ?? [],
});

export const featuredBlogs: BlogCardEntry[] = [
  createBlogEntry({
    slug: 'sspai-think',
    name: '少数派 · Think 的技术备忘与长期观察',
    domain: 'sspai.com/author/think/notes-and-observations',
    href: 'https://sspai.com/author/think',
    shortCode: 'TH',
    primaryTag: '技术',
    summary: '深度思考技术与社会的交叉地带，关注工具、方法论与长期写作实践，适合订阅型阅读。',
    profile:
      '这类博客适合放在目录首页的推荐位：更新频率稳定、选题跨度足够宽，同时文章之间有持续的个人方法论线索，适合作为长期订阅对象。',
    highlights: ['工具与效率方法论', '技术与社会议题交叉', '长文密度高'],
    subTags: [
      '效率',
      '方法论',
      '写作',
      '工具',
      '生产力',
      '笔记',
      '长期主义',
      '播客',
      'workflow',
      '观察',
    ],
    joinedAt: '2021-04-09',
    joinedLabel: '2021.04',
    updatedLabel: '2 天前更新',
    updatedTone: 'emerald',
    articleCount: '234',
    visitCount: '1.2k',
    tone: 'blue',
    rssUrl: 'https://sspai.com/feed',
    sitemapUrl: 'https://sspai.com/sitemap.xml',
    featured: true,
  }),
  createBlogEntry({
    slug: 'codingnow',
    name: '云风 Blog',
    domain: 'blog.codingnow.com',
    href: 'https://blog.codingnow.com',
    shortCode: 'YF',
    primaryTag: '技术',
    summary: '底层实现与运行时札记。',
    profile:
      '它的价值不在于短平快的信息摘录，而在于作者持续输出底层实现、语言设计和工程取舍，适合已经有开发经验、希望继续往深处看的读者。',
    highlights: ['底层实现与运行时', '游戏引擎经验', '偏技术纵深阅读'],
    subTags: ['Lua'],
    joinedAt: '2020-11-18',
    joinedLabel: '2020.11',
    updatedLabel: '5 天前更新',
    updatedTone: 'emerald',
    articleCount: '891',
    visitCount: '3.8k',
    tone: 'blue',
    rssUrl: 'https://blog.codingnow.com/atom.xml',
  }),
  createBlogEntry({
    slug: 'idealand',
    name: '程序员的理想乡',
    domain: 'idealand.xyz/journal',
    href: 'https://idealand.xyz',
    shortCode: 'LX',
    primaryTag: '生活',
    summary: '写时间、阅读和缓慢生活。',
    profile:
      '这类站点更偏生活化与知识管理，适合作为技术目录中的缓冲层，让首页不只堆叠硬核技术内容，也能呈现长期写作者的个人系统建设。',
    highlights: ['知识管理与读书笔记', '生活系统建设', '技术随笔更具个人性'],
    subTags: [],
    joinedAt: '2022-07-03',
    joinedLabel: '2022.07',
    updatedLabel: '暂无更新记录',
    updatedTone: 'stone',
    visitCount: '456',
    tone: 'emerald',
    sitemapUrl: 'https://idealand.xyz/sitemap.xml',
  }),
  createBlogEntry({
    slug: 'designnotes',
    name: '设计备忘录 Weekly',
    domain: 'designnotes.me/archive/weekly-observation',
    href: 'https://designnotes.me',
    shortCode: 'SJ',
    primaryTag: '设计',
    summary: '产品设计师的日常观察，记录界面语言、色彩心理学和用户体验研究的点滴思考。',
    profile:
      '相比纯视觉展示，这类博客更适合以“设计判断”来组织内容，卡片里只放入口信息，详情页再承接更完整的设计视角和阅读理由。',
    highlights: ['界面语言与产品设计', '用户体验观察', '适合跨学科读者'],
    subTags: ['产品', '交互', '体验', '可用性', '界面'],
    joinedAt: '2023-02-14',
    joinedLabel: '2023.02',
    updatedLabel: '1 天前更新',
    updatedTone: 'emerald',
    articleCount: '52',
    visitCount: '890',
    tone: 'amber',
    rssUrl: 'https://designnotes.me/feed.xml',
    sitemapUrl: 'https://designnotes.me/sitemap.xml',
  }),
  createBlogEntry({
    slug: 'reader-wanderings',
    name: '书架边的漫游者：人文阅读周报与札记',
    domain: 'reader.wanderings.cc/weekly-letter',
    href: 'https://reader.wanderings.cc',
    shortCode: 'DS',
    primaryTag: '人文',
    summary: '横跨哲学、社会学与文学的阅读札记，用专题方式连接不同时代的思想线索。',
    profile:
      '它承担的是目录中的人文阅读入口，不追求高频更新，而是通过相对稳定的专题式内容提供长周期价值，因此更适合在详情页补充作者风格和阅读方向。',
    highlights: ['哲学与社会学阅读', '专题式书评', '更新频率低但沉淀强'],
    subTags: ['读书', '哲学', '文学', '社会学'],
    joinedAt: '2021-09-27',
    joinedLabel: '2021.09',
    updatedLabel: '28 天前更新',
    updatedTone: 'amber',
    articleCount: '143',
    visitCount: '2.1k',
    tone: 'stone',
    rssUrl: 'https://reader.wanderings.cc/feed.xml',
  }),
  createBlogEntry({
    slug: 'indie-jhack',
    name: '独立开发手记',
    domain: 'indie.jhack.dev/launch-log',
    href: 'https://indie.jhack.dev',
    shortCode: 'ID',
    primaryTag: '独立开发',
    summary: '一个人从零发布三款 App 的完整记录。',
    profile:
      '这类博客非常适合做详情页承接，因为读者通常不是只想知道它是什么，而是希望判断它是否真的覆盖产品、商业化和增长这些连续主题。',
    highlights: ['产品发布与复盘', '营销与增长实验', '独立开发过程记录'],
    subTags: ['产品', '增长', '营销', '收入', '复盘', 'SaaS', '出海'],
    joinedAt: '2024-01-08',
    joinedLabel: '2024.01',
    updatedLabel: '暂无更新记录',
    updatedTone: 'stone',
    visitCount: '4.7k',
    tone: 'red',
  }),
];
