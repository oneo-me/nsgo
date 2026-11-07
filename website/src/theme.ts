export type ThemePreference = 'system' | 'light' | 'dark';
export type Theme = 'light' | 'dark';

export const THEME_STORAGE_KEY = 'nsgo-theme';

const darkColor = '#181818';
const lightColor = '#f3f3f3';

export function readPreference(): ThemePreference {
  try {
    const stored = localStorage.getItem(THEME_STORAGE_KEY);
    if (stored === 'light' || stored === 'dark' || stored === 'system') return stored;
  } catch {
    // Storage can be unavailable; follow the browser.
  }
  return 'system';
}

export function systemPrefersDark(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

export function resolveTheme(preference: ThemePreference, systemDark: boolean): Theme {
  return preference === 'system' ? (systemDark ? 'dark' : 'light') : preference;
}

export function applyTheme(theme: Theme): void {
  const root = document.documentElement;
  root.dataset.theme = theme;
  root.style.colorScheme = theme;
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute('content', theme === 'dark' ? darkColor : lightColor);
}

export function storePreference(preference: ThemePreference): void {
  try {
    localStorage.setItem(THEME_STORAGE_KEY, preference);
  } catch {
    // The appearance still applies for this visit.
  }
}
