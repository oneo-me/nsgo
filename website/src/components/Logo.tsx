import darkLogo from '../assets/logo-dark.svg?url';
import lightLogo from '../assets/logo-light.svg?url';
import type { Theme } from '../hooks/useTheme';

export function Logo({
  theme,
  size = 26,
  className,
}: {
  theme: Theme;
  size?: number;
  className?: string;
}) {
  return (
    <img
      className={className}
      src={theme === 'dark' ? darkLogo : lightLogo}
      width={size}
      height={size}
      alt=""
      draggable={false}
    />
  );
}
