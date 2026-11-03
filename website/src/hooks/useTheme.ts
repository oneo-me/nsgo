import { useEffect, useState } from 'react';
import {
  applyTheme,
  readPreference,
  resolveTheme,
  storePreference,
  systemPrefersDark,
  type Theme,
  type ThemePreference,
} from '../theme';

export type { Theme, ThemePreference } from '../theme';

export function useTheme(): {
  preference: ThemePreference;
  theme: Theme;
  setPreference: (preference: ThemePreference) => void;
} {
  const [preference, setPreference] = useState<ThemePreference>(readPreference);
  const [systemDark, setSystemDark] = useState(systemPrefersDark);

  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const onChange = (event: MediaQueryListEvent) => setSystemDark(event.matches);
    media.addEventListener('change', onChange);
    return () => media.removeEventListener('change', onChange);
  }, []);

  const theme = resolveTheme(preference, systemDark);

  useEffect(() => {
    applyTheme(theme);
    storePreference(preference);
  }, [preference, theme]);

  return { preference, theme, setPreference };
}
