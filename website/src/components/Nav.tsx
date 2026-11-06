import { useEffect, useState } from 'react';
import type { Copy, Lang } from '../copy';
import { links } from '../copy';
import type { Theme, ThemePreference } from '../hooks/useTheme';
import { Icon } from './Icon';
import { LangSwitch } from './LangSwitch';
import { Logo } from './Logo';
import { ThemeSwitch } from './ThemeSwitch';

export function Nav({
  theme,
  preference,
  setPreference,
  lang,
  setLang,
  copy,
}: {
  theme: Theme;
  preference: ThemePreference;
  setPreference: (preference: ThemePreference) => void;
  lang: Lang;
  setLang: (lang: Lang) => void;
  copy: Copy;
}) {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false);
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [open]);

  return (
    <header className="nav">
      <div className="container nav__inner">
        <a className="brand" href="#top" aria-label="NSGo">
          <Logo theme={theme} size={26} className="brand__logo" />
          <span className="brand__name">NSGo</span>
          <span className="brand__tag">Native UI in Go</span>
        </a>

        <nav
          id="site-menu"
          className={`nav__links${open ? ' is-open' : ''}`}
          aria-label={copy.nav.primary}
        >
          {copy.nav.links.map((link) => {
            const external = link.href === links.gallery;
            return (
              <a
                key={link.href}
                href={link.href}
                {...(external ? { target: '_blank', rel: 'noopener noreferrer' } : {})}
                onClick={() => setOpen(false)}
              >
                {link.label}
              </a>
            );
          })}
        </nav>

        <div className="nav__actions">
          <ThemeSwitch preference={preference} setPreference={setPreference} copy={copy} />
          <LangSwitch lang={lang} setLang={setLang} copy={copy} />
          <a
            className="nav__github"
            href={links.repo}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={copy.nav.github}
            title={copy.nav.github}
          >
            <Icon name="github" size={17} />
            <span>{copy.nav.github}</span>
          </a>
          <button
            type="button"
            className="nav__burger"
            aria-expanded={open}
            aria-controls="site-menu"
            aria-label={open ? copy.nav.close : copy.nav.menu}
            title={open ? copy.nav.close : copy.nav.menu}
            onClick={() => setOpen((value) => !value)}
          >
            <Icon name={open ? 'close' : 'menu'} size={18} />
          </button>
        </div>
      </div>
    </header>
  );
}
