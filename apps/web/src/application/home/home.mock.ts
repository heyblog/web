import type { HomeMockMode, HomeSiteCard, HomeView } from '@/application/home/home.shared';

export interface HomeMockState {
  home: HomeView | null;
  unavailable: boolean;
}

const baseJoinedAt = '2025-01-15T00:00:00Z';
const baseUpdatedAt = '2026-08-15T00:00:00Z';

const homeMockCards: HomeSiteCard[] = [
  createMockCard(1, {
    customId: 'field-notes',
    name: '旷野札记',
    summary: '记录自然观察、长途徒步与日常生活里值得慢慢回看的片段。',
    accessScope: 'ALL',
    topics: [
      { name: '生活', slug: 'life', role: 'PRIMARY' },
      { name: '摄影', slug: 'photography', role: 'SECONDARY' },
    ],
    defaultFeed: feed('RSS'),
    sitemapUrl: 'https://field-notes.example/sitemap.xml',
  }),
  createMockCard(2, {
    name: '代码与茶',
    summary: '关于软件工程、开源维护和一杯茶之间的思考。',
    accessScope: 'CN_ONLY',
    topics: [{ name: '技术', slug: 'technology', role: 'PRIMARY' }],
    defaultFeed: feed('ATOM'),
  }),
  createMockCard(3, {
    customId: 'borderless-journal',
    name: 'Borderless Journal',
    summary: '',
    accessScope: 'GLOBAL_ONLY',
    defaultFeed: feed('JSON'),
    sitemapUrl: 'https://borderless-journal.example/sitemap-index.xml',
  }),
  createMockCard(4, {
    name: '一间很长名字的独立博客与它持续更新的公开写作实验',
    summary: '这是一段用于检验卡片长文本、换行和截断状态的描述，同时保持内容仍然像真实博客资料。',
    host: 'a-deliberately-long-independent-blog-domain.example',
    homepageUrl: 'https://a-deliberately-long-independent-blog-domain.example/',
    topics: [
      { name: '人文', slug: 'humanities', role: 'PRIMARY' },
      { name: '阅读', slug: 'reading', role: 'SECONDARY' },
      { name: '写作', slug: 'writing', role: 'SECONDARY' },
      { name: '独立出版', slug: 'independent-publishing', role: 'SECONDARY' },
      { name: '城市观察', slug: 'urban-observation', role: 'SECONDARY' },
      { name: '文化研究', slug: 'cultural-studies', role: 'SECONDARY' },
      { name: '地方历史', slug: 'local-history', role: 'SECONDARY' },
      { name: '书籍设计', slug: 'book-design', role: 'SECONDARY' },
      { name: '纸本阅读', slug: 'print-reading', role: 'SECONDARY' },
      { name: '非虚构写作', slug: 'nonfiction-writing', role: 'SECONDARY' },
      { name: '公共空间研究', slug: 'public-space-studies', role: 'SECONDARY' },
      { name: '视觉文化研究', slug: 'visual-culture-studies', role: 'SECONDARY' },
      { name: '独立杂志收藏', slug: 'independent-magazines', role: 'SECONDARY' },
      { name: '地方知识整理', slug: 'local-knowledge', role: 'SECONDARY' },
    ],
  }),
  createMockCard(5, {
    name: '离线花园',
    summary: '低频更新的个人数字花园。',
    accessScope: 'CN_ONLY',
    topics: [{ name: '数字花园', slug: 'digital-garden', role: 'PRIMARY' }],
    warnings: [
      { name: '访问较慢', slug: 'slow-access', description: '部分网络环境下访问速度较慢。' },
    ],
    defaultFeed: feed('RSS'),
  }),
  createMockCard(6, {
    customId: 'open-circuit',
    name: 'Open Circuit',
    summary: 'Hardware notes, tiny tools, and experiments in public.',
    accessScope: 'GLOBAL_ONLY',
    topics: [
      { name: '技术', slug: 'technology', role: 'PRIMARY' },
      { name: '硬件', slug: 'hardware', role: 'SECONDARY' },
    ],
    warnings: [{ name: '仅英文', slug: 'english-only', description: '内容主要使用英文写作。' }],
    sitemapUrl: 'https://open-circuit.example/sitemap.xml',
  }),
  createMockCard(7, {
    name: '岛屿来信',
    summary: '从海边寄出的生活、电影与地方文化随笔。',
    topics: [
      { name: '生活', slug: 'life', role: 'PRIMARY' },
      { name: '电影', slug: 'film', role: 'SECONDARY' },
    ],
    defaultFeed: feed('ATOM'),
    sitemapUrl: 'https://island-letters.example/sitemap.xml',
  }),
  createMockCard(8, {
    name: '像素之外',
    summary: '界面设计、无障碍与产品细节记录。',
    topics: [
      { name: '设计', slug: 'design', role: 'PRIMARY' },
      { name: '无障碍', slug: 'accessibility', role: 'SECONDARY' },
    ],
  }),
  createMockCard(9, {
    customId: 'small-data',
    name: 'Small Data Lab',
    summary: 'Personal analytics without surveillance.',
    topics: [{ name: '数据', slug: 'data', role: 'PRIMARY' }],
    defaultFeed: feed('JSON'),
  }),
  createMockCard(10, {
    name: '纸上旅行',
    summary: '城市漫步、建筑观察与旅行手记。',
    accessScope: 'CN_ONLY',
    topics: [{ name: '旅行', slug: 'travel', role: 'PRIMARY' }],
    sitemapUrl: 'https://paper-travel.example/sitemap.xml',
  }),
  createMockCard(11, {
    name: 'Plain Text',
    summary: 'A quiet blog with no categories or discoverable resources.',
    accessScope: 'GLOBAL_ONLY',
  }),
  createMockCard(12, {
    name: '晨间广播',
    summary: '每天一篇短记录，偶尔附带声音和照片。',
    topics: [
      { name: '随笔', slug: 'essay', role: 'PRIMARY' },
      { name: '声音', slug: 'audio', role: 'SECONDARY' },
    ],
    warnings: [
      { name: '包含音频', slug: 'audio-content', description: '部分文章包含自动加载的音频资源。' },
    ],
    defaultFeed: feed('RSS'),
    sitemapUrl: 'https://morning-radio.example/sitemap.xml',
  }),
];

export function parseHomeMockMode(value: string | null, enabled: boolean): HomeMockMode | null {
  return enabled && (value === 'cards' || value === 'empty' || value === 'error') ? value : null;
}

export function createHomeMockState(mode: HomeMockMode): HomeMockState {
  if (mode === 'error') {
    return { home: null, unavailable: true };
  }

  const sites = mode === 'cards' ? homeMockCards : [];
  return {
    home: {
      siteCount: sites.length,
      announcement: null,
      sites,
    },
    unavailable: false,
  };
}

function createMockCard(
  index: number,
  overrides: Partial<HomeSiteCard> & Pick<HomeSiteCard, 'name' | 'summary'>,
): HomeSiteCard {
  const host = `mock-${index}.example`;
  return {
    shortId: `Mock${index.toString().padStart(5, '0')}`,
    customId: null,
    host,
    homepageUrl: `https://${host}/`,
    accessScope: 'ALL',
    joinedAt: baseJoinedAt,
    updatedAt: baseUpdatedAt,
    topics: [],
    warnings: [],
    defaultFeed: null,
    sitemapUrl: null,
    ...overrides,
  };
}

function feed(format: 'RSS' | 'ATOM' | 'JSON') {
  const extension = format === 'JSON' ? 'json' : 'xml';
  return {
    name: `${format} Feed`,
    url: `https://feeds.example/${format.toLowerCase()}.${extension}`,
    format,
  } as const;
}
