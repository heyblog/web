(() => {
  const root = document.documentElement;
  const storageKey = root.dataset.themeStorageKey ?? 'heyblog-theme';
  let storedTheme;

  try {
    storedTheme = localStorage.getItem(storageKey);
  } catch {
    storedTheme = null;
  }

  const systemTheme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  const theme = storedTheme === 'light' || storedTheme === 'dark' ? storedTheme : systemTheme;

  root.dataset.theme = theme;
  root.style.colorScheme = theme;
})();
