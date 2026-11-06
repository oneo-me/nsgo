import { CodeSection } from './components/CodeSection';
import { EngineSection } from './components/EngineSection';
import { Footer } from './components/Footer';
import { Hero } from './components/Hero';
import { Nav } from './components/Nav';
import { OpenSection } from './components/OpenSection';
import { PlatformsSection } from './components/PlatformsSection';
import { ToolchainSection } from './components/ToolchainSection';
import { useLanguage } from './hooks/useLanguage';
import { useTheme } from './hooks/useTheme';

export default function App() {
  const { preference, theme, setPreference } = useTheme();
  const { lang, copy, setLang } = useLanguage();

  return (
    <>
      <a className="skip-link" href="#main">
        {copy.nav.skip}
      </a>
      <Nav
        theme={theme}
        preference={preference}
        setPreference={setPreference}
        lang={lang}
        setLang={setLang}
        copy={copy}
      />
      <main id="main">
        <Hero copy={copy} />
        <EngineSection copy={copy} />
        <PlatformsSection copy={copy} />
        <CodeSection copy={copy} />
        <ToolchainSection copy={copy} />
        <OpenSection copy={copy} />
      </main>
      <Footer copy={copy} theme={theme} />
    </>
  );
}
