// The appearance the site shows. `nsgo-theme` holds light, dark or system;
// absent is light. The layout's inline script applies it before the first
// paint; this module switches it. The gallery's loader reads the same key.

export type ThemePreference = 'light' | 'dark' | 'system';
export type Theme = 'light' | 'dark';

const key = 'nsgo-theme';
const media = window.matchMedia('(prefers-color-scheme: dark)');

export function readPreference(): ThemePreference {
  try {
    const stored = localStorage.getItem(key);
    if (stored === 'dark' || stored === 'system') return stored;
  } catch {
    // Storage can be unavailable; show light.
  }
  return 'light';
}

export function currentTheme(): Theme {
  return document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light';
}

function apply(): void {
  const preference = readPreference();
  const theme: Theme = preference === 'system' ? (media.matches ? 'dark' : 'light') : preference;
  const root = document.documentElement;
  root.dataset.theme = theme;
  root.style.colorScheme = theme;
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute('content', theme === 'dark' ? '#0b0c0f' : '#ffffff');
  document.dispatchEvent(new CustomEvent<ThemePreference>('nsgo-theme', { detail: preference }));
}

export function setPreference(preference: ThemePreference): void {
  try {
    localStorage.setItem(key, preference);
  } catch {
    // The appearance still applies for this visit.
  }
  apply();
}

media.addEventListener('change', () => {
  if (readPreference() === 'system') apply();
});
