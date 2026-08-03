const markdownExtensions = ['.mdx', '.md'];

function markdownDirectory(path) {
  return {
    kind: 'directory',
    path,
    extensions: markdownExtensions,
    pattern: markdownExtensions.map((extension) => `**/*${extension}`),
  };
}

export const contentSources = {
  members: {
    kind: 'file',
    path: 'members.json',
  },
  blogs: markdownDirectory('blogs'),
  docs: markdownDirectory('docs'),
  pages: markdownDirectory('pages'),
};
