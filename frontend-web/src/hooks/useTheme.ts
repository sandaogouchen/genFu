import { useState, useEffect } from 'react';

type Theme = 'light' | 'dark';
const THEME_STORAGE_KEY = 'genfu.ui.theme';
const LEGACY_THEME_STORAGE_KEY = 'theme';

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(() => {
    try {
      const savedTheme =
        (localStorage.getItem(THEME_STORAGE_KEY) ??
          localStorage.getItem(LEGACY_THEME_STORAGE_KEY)) as Theme | null;
      if (savedTheme === 'light' || savedTheme === 'dark') {
        return savedTheme;
      }
    } catch {
      void 0;
    }

    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  });

  useEffect(() => {
    const html = document.documentElement;
    html.classList.remove('light');
    html.classList.toggle('dark', theme === 'dark');
    try {
      localStorage.setItem(THEME_STORAGE_KEY, theme);
    } catch {
      void 0;
    }
  }, [theme]);

  const toggleTheme = () => {
    setTheme(prevTheme => prevTheme === 'light' ? 'dark' : 'light');
  };

  return {
    theme,
    toggleTheme,
    isDark: theme === 'dark'
  };
}
