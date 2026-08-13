const githubUsernamePattern = /^[a-z\d](?:[a-z\d-]{0,37}[a-z\d])?$/iu;

export function isGithubUsername(value: string): boolean {
  return githubUsernamePattern.test(value);
}

export function getGithubAvatarPath(username: string): string {
  if (!isGithubUsername(username)) {
    throw new Error(`Invalid GitHub username: ${username}`);
  }

  return `/media/github-avatar/${encodeURIComponent(username)}.png`;
}
