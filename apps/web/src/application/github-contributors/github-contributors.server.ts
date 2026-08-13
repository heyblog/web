import { loadWebServerConfig } from '@/config.server';

import {
  createGithubContributorsReader,
  type GithubContributorsSnapshot,
} from './github-contributors';

let readContributors: (() => Promise<GithubContributorsSnapshot | undefined>) | undefined;

export function readGithubContributors(): Promise<GithubContributorsSnapshot | undefined> {
  if (!readContributors) {
    const { githubToken } = loadWebServerConfig();
    readContributors = createGithubContributorsReader({ token: githubToken });
  }

  return readContributors();
}
