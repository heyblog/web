export interface GithubContributor {
  login: string;
  avatarUrl: string;
  htmlUrl: string;
  contributions: number;
  repositoryCount: number;
}

export interface GithubContributorsSnapshot {
  contributors: GithubContributor[];
  fetchedAt: number;
  stale: boolean;
}

export interface GithubContributorsConfig {
  token?: string;
  organization?: string;
  fetch?: typeof fetch;
  now?: () => number;
  freshTtlMs?: number;
  staleTtlMs?: number;
  timeoutMs?: number;
  maxPages?: number;
  concurrency?: number;
}

interface GithubRepository {
  name: string;
  archived?: boolean;
  fork?: boolean;
  topics?: string[];
  description?: string | null;
  homepage?: string | null;
}

interface GithubContributorResponse {
  login?: string;
  avatar_url?: string;
  html_url?: string;
  contributions?: number;
  type?: string;
}

const defaultOrganization = 'heyblog';
const githubApiOrigin = 'https://api.github.com';
const oldRepositoryNames = new Set(['v1', 'v2', 'blog-daohang']);
const oldBrandPattern = /(?:zhblogs|集博栈)/iu;

export function createGithubContributorsReader(
  config: GithubContributorsConfig = {},
): () => Promise<GithubContributorsSnapshot | undefined> {
  const fetchGithub = config.fetch ?? fetch;
  const now = config.now ?? Date.now;
  const organization = config.organization ?? defaultOrganization;
  const freshTtlMs = config.freshTtlMs ?? 30 * 60 * 1_000;
  const staleTtlMs = config.staleTtlMs ?? 24 * 60 * 60 * 1_000;
  const timeoutMs = config.timeoutMs ?? 10_000;
  const maxPages = config.maxPages ?? 20;
  const concurrency = config.concurrency ?? 4;
  let cached: GithubContributorsSnapshot | undefined;
  let pending: Promise<GithubContributorsSnapshot | undefined> | undefined;

  if (freshTtlMs <= 0 || staleTtlMs <= 0 || timeoutMs <= 0 || maxPages <= 0 || concurrency <= 0) {
    throw new Error('GitHub contributor cache limits must be positive.');
  }

  return async () => {
    const currentTime = now();

    if (cached && cached.fetchedAt + freshTtlMs > currentTime) {
      return { ...cached, stale: false };
    }

    if (!pending) {
      pending = loadSnapshot({
        fetchGithub,
        now,
        organization,
        token: config.token,
        timeoutMs,
        maxPages,
        concurrency,
      })
        .then((snapshot) => {
          if (snapshot) {
            cached = snapshot;
          }
          return snapshot;
        })
        .finally(() => {
          pending = undefined;
        });
    }

    const loaded = await pending;

    if (loaded) {
      return { ...loaded, stale: false };
    }

    if (cached && cached.fetchedAt + freshTtlMs + staleTtlMs > now()) {
      return { ...cached, stale: true };
    }

    return undefined;
  };
}

interface LoadSnapshotOptions {
  fetchGithub: typeof fetch;
  now: () => number;
  organization: string;
  token?: string;
  timeoutMs: number;
  maxPages: number;
  concurrency: number;
}

async function loadSnapshot(
  options: LoadSnapshotOptions,
): Promise<GithubContributorsSnapshot | undefined> {
  try {
    const repositories = await fetchAllRepositories(options);
    const eligibleRepositories = repositories.filter(isEligibleRepository);
    let failedRepositories = 0;
    const repositoryContributors = await mapWithConcurrency(
      eligibleRepositories,
      options.concurrency,
      async (repository) => {
        try {
          return await fetchAllContributors(repository.name, options);
        } catch {
          failedRepositories += 1;
          return [];
        }
      },
    );
    if (eligibleRepositories.length > 0 && failedRepositories === eligibleRepositories.length) {
      throw new Error('GitHub contributor requests failed for every repository.');
    }
    const contributors = aggregateContributors(repositoryContributors.flat());

    return {
      contributors,
      fetchedAt: options.now(),
      stale: false,
    };
  } catch {
    return undefined;
  }
}

async function fetchAllRepositories(options: LoadSnapshotOptions): Promise<GithubRepository[]> {
  const repositories: GithubRepository[] = [];

  for (let page = 1; page <= options.maxPages; page += 1) {
    const result = await githubJson<GithubRepository[]>(
      `${githubApiOrigin}/orgs/${encodeURIComponent(options.organization)}/repos?type=public&per_page=100&page=${page}`,
      options,
    );
    repositories.push(...result);

    if (result.length < 100) {
      break;
    }
  }

  return repositories;
}

async function fetchAllContributors(
  repository: string,
  options: LoadSnapshotOptions,
): Promise<GithubContributorResponse[]> {
  const contributors: GithubContributorResponse[] = [];

  for (let page = 1; page <= options.maxPages; page += 1) {
    const result = await githubJson<GithubContributorResponse[]>(
      `${githubApiOrigin}/repos/${encodeURIComponent(options.organization)}/${encodeURIComponent(repository)}/contributors?anon=false&per_page=100&page=${page}`,
      options,
    );
    contributors.push(...result);

    if (result.length < 100) {
      break;
    }
  }

  return contributors;
}

async function githubJson<T>(url: string, options: LoadSnapshotOptions): Promise<T> {
  const headers = new Headers({
    Accept: 'application/vnd.github+json',
    'User-Agent': 'HeyBlog-member-contributors/1.0',
    'X-GitHub-Api-Version': '2022-11-28',
  });

  if (options.token) {
    headers.set('Authorization', `Bearer ${options.token}`);
  }

  const response = await options.fetchGithub(url, {
    headers,
    signal: AbortSignal.timeout(options.timeoutMs),
  });

  if (!response.ok) {
    throw new Error(`GitHub API returned ${response.status}.`);
  }

  return (await response.json()) as T;
}

export function isEligibleRepository(repository: GithubRepository): boolean {
  const name = repository.name.trim().toLocaleLowerCase('en-US');
  const topics = repository.topics ?? [];
  const oldBrandText = [repository.description ?? '', repository.homepage ?? '', ...topics].join(
    ' ',
  );

  return (
    !repository.archived &&
    !repository.fork &&
    !oldRepositoryNames.has(name) &&
    !oldBrandPattern.test(oldBrandText)
  );
}

function aggregateContributors(items: GithubContributorResponse[]): GithubContributor[] {
  const aggregated = new Map<string, GithubContributor>();

  for (const item of items) {
    const contributions = item.contributions;

    if (
      !item.login ||
      item.type === 'Bot' ||
      !item.avatar_url ||
      !item.html_url ||
      typeof contributions !== 'number' ||
      !Number.isFinite(contributions) ||
      contributions <= 0
    ) {
      continue;
    }

    const key = item.login.toLocaleLowerCase('en-US');
    const existing = aggregated.get(key);

    if (existing) {
      existing.contributions += contributions;
      existing.repositoryCount += 1;
      continue;
    }

    aggregated.set(key, {
      login: item.login,
      avatarUrl: item.avatar_url,
      htmlUrl: item.html_url,
      contributions,
      repositoryCount: 1,
    });
  }

  return [...aggregated.values()].sort(
    (left, right) =>
      right.contributions - left.contributions ||
      left.login.localeCompare(right.login, 'en-US', { sensitivity: 'base' }),
  );
}

async function mapWithConcurrency<T, R>(
  values: T[],
  concurrency: number,
  mapper: (value: T) => Promise<R>,
): Promise<R[]> {
  const results = new Array<R>(values.length);
  let nextIndex = 0;

  async function worker(): Promise<void> {
    while (nextIndex < values.length) {
      const index = nextIndex;
      nextIndex += 1;
      results[index] = await mapper(values[index]);
    }
  }

  await Promise.all(Array.from({ length: Math.min(concurrency, values.length) }, () => worker()));
  return results;
}
