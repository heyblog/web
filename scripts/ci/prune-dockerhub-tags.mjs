#!/usr/bin/env node

const API_BASE_URL = "https://hub.docker.com/v2";
const DEFAULT_PROTECTED_TAGS = ["main", "latest"];

function readRequiredEnv(name) {
  const value = process.env[name];
  if (!value) {
    throw new Error(`${name} is required`);
  }
  return value;
}

function readKeepCount() {
  const value = Number.parseInt(process.env.DOCKERHUB_KEEP_TAGS || "10", 10);
  if (!Number.isInteger(value) || value < 0) {
    throw new Error("DOCKERHUB_KEEP_TAGS must be a non-negative integer");
  }
  return value;
}

function readProtectedTags() {
  const tags = process.env.DOCKERHUB_PROTECTED_TAGS
    ? process.env.DOCKERHUB_PROTECTED_TAGS.split(",")
    : DEFAULT_PROTECTED_TAGS;
  return new Set(tags.map((tag) => tag.trim()).filter(Boolean));
}

async function requestJson(url, options = {}) {
  const response = await fetch(url, options);
  if (!response.ok) {
    const body = await response.text();
    throw new Error(
      `${options.method || "GET"} ${url} failed: ${response.status} ${body}`,
    );
  }
  if (response.status === 204) {
    return null;
  }
  return response.json();
}

async function login(username, password) {
  const response = await requestJson(`${API_BASE_URL}/users/login/`, {
    method: "POST",
    headers: {
      "content-type": "application/json",
    },
    body: JSON.stringify({ username, password }),
  });
  if (!response?.token) {
    throw new Error("Docker Hub login did not return a token");
  }
  return response.token;
}

async function listTags(namespace, repository, token) {
  const tags = [];
  let nextUrl = `${API_BASE_URL}/repositories/${encodeURIComponent(namespace)}/${encodeURIComponent(
    repository,
  )}/tags?page_size=100`;

  while (nextUrl) {
    const page = await requestJson(nextUrl, {
      headers: {
        authorization: `JWT ${token}`,
      },
    });
    tags.push(...(page.results || []));
    nextUrl = page.next;
  }

  return tags;
}

async function deleteTag(namespace, repository, tag, token) {
  await requestJson(
    `${API_BASE_URL}/repositories/${encodeURIComponent(namespace)}/${encodeURIComponent(
      repository,
    )}/tags/${encodeURIComponent(tag)}/`,
    {
      method: "DELETE",
      headers: {
        authorization: `JWT ${token}`,
      },
    },
  );
}

function toTimestamp(tag) {
  return Date.parse(tag.last_updated || tag.tag_last_pushed || "") || 0;
}

function selectTagsToDelete(tags, keepCount, protectedTags, currentTag) {
  const historyTags = tags
    .filter(
      (tag) => typeof tag?.name === "string" && !protectedTags.has(tag.name),
    )
    .sort((left, right) => toTimestamp(right) - toTimestamp(left));

  const retained = new Set();
  if (
    currentTag &&
    !protectedTags.has(currentTag) &&
    historyTags.some((tag) => tag.name === currentTag)
  ) {
    retained.add(currentTag);
  }

  for (const tag of historyTags) {
    if (retained.size >= keepCount) {
      break;
    }
    retained.add(tag.name);
  }

  return historyTags.filter((tag) => !retained.has(tag.name));
}

async function main() {
  const username = readRequiredEnv("DOCKERHUB_USERNAME");
  const password = readRequiredEnv("DOCKERHUB_TOKEN");
  const namespace = process.env.DOCKERHUB_NAMESPACE || username;
  const repository = readRequiredEnv("DOCKERHUB_REPOSITORY");
  const keepCount = readKeepCount();
  const protectedTags = readProtectedTags();
  const currentTag = process.env.IMAGE_TAG;
  const dryRun = process.env.DOCKERHUB_PRUNE_DRY_RUN === "true";

  const token = await login(username, password);
  const tags = await listTags(namespace, repository, token);
  const tagsToDelete = selectTagsToDelete(
    tags,
    keepCount,
    protectedTags,
    currentTag,
  );

  console.log(
    `${namespace}/${repository}: keep ${keepCount} history tags, protect ${Array.from(
      protectedTags,
    ).join(",")}, delete ${tagsToDelete.length} old tags`,
  );

  for (const tag of tagsToDelete) {
    if (dryRun) {
      console.log(`dry-run delete ${namespace}/${repository}:${tag.name}`);
      continue;
    }
    await deleteTag(namespace, repository, tag.name, token);
    console.log(`deleted ${namespace}/${repository}:${tag.name}`);
  }
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
