import assert from 'node:assert/strict';
import test from 'node:test';

import { llmsText, robotsText } from '../src/shared/crawlers.ts';
import { crawlerDisallowPatterns } from '../src/shared/indexing.ts';

test('search agents retain all private exclusions while training agents deny all paths', () => {
  const groups = robotsText().split('\n\n');
  for (const agent of ['*', 'OAI-SearchBot', 'Claude-SearchBot']) {
    const group = groups.find((value) => value.startsWith(`User-agent: ${agent}\n`));
    assert.ok(group);
    assert.match(group, /Allow: \/\n/);
    for (const path of crawlerDisallowPatterns) assert.ok(group.includes(`Disallow: ${path}`));
    assert.ok(!group.includes('Disallow: /og'));
  }
  for (const agent of ['GPTBot', 'ClaudeBot', 'Google-Extended', 'CCBot']) {
    assert.ok(groups.includes(`User-agent: ${agent}\nDisallow: /`));
  }
  assert.match(robotsText(), /Sitemap: https:\/\/www\.heyblog\.net\/sitemap\.xml\n$/);
});

test('AI navigation describes ownership and links public content without internal interfaces', () => {
  const text = llmsText();
  assert.match(text, /第三方博客及其文章的权利归原作者所有/);
  assert.match(text, /不授予.*模型训练许可/);
  assert.ok(text.includes('/sitemap.xml'));
  assert.doesNotMatch(text, /\/management|\/auth|\/api\/|API_WEB_TOKEN|WEB_API_BASE_URL/);
});

// Interpret REP prefix/wildcard/end-anchor semantics independently of the policy generator.
test('random-page rules exclude query/slash variants without blocking go-prefixed short IDs', () => {
  const group = robotsText()
    .split('\n\n')
    .find((value) => value.startsWith('User-agent: *\n'));
  assert.ok(group);
  const patterns = group
    .split('\n')
    .filter((line) => line.startsWith('Disallow: '))
    .map((line) => line.slice(10));
  const blocked = (path: string) =>
    patterns.some((pattern) => {
      const anchored = pattern.endsWith('$');
      const body = anchored ? pattern.slice(0, -1) : pattern;
      const expression = body
        .split('*')
        .map((part) => part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
        .join('.*');
      return new RegExp(`^${expression}${anchored ? '$' : ''}`).test(path);
    });
  for (const path of [
    '/site/go',
    '/site/go/',
    '/site/go?preview=true',
    '/site/go/?preview=true',
    '/login?next=/dashboard',
    '/management/tags',
  ])
    assert.equal(blocked(path), true, path);
  for (const path of [
    '/site/gopherABC',
    '/site/go1234567',
    '/site',
    '/site?page=2',
    '/og/site/gopherABC.png',
  ])
    assert.equal(blocked(path), false, path);
});
