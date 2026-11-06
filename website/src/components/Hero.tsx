import type { Copy } from '../copy';
import { links } from '../copy';
import { CopyButton } from './CopyButton';
import { Icon } from './Icon';

export function Hero({ copy }: { copy: Copy }) {
  return (
    <section className="hero" id="top">
      <div className="container hero__inner">
        <div className="hero__copy">
          <span className="eyebrow">
            <i aria-hidden="true" />
            {copy.hero.eyebrow}
          </span>

          <h1>
            {copy.hero.titleTop}
            <br />
            {copy.hero.titleBefore}
            <span className="accent">{copy.hero.titleAccent}</span>
            {copy.hero.titleAfter}
          </h1>

          <p className="hero__lead">{copy.hero.lead}</p>

          <ul className="hero__points">
            {copy.hero.points.map((point) => (
              <li className="hero__point" key={point.title}>
                <span className="hero__point-dot" aria-hidden="true" />
                <span>
                  <strong>{point.title}</strong> — {point.body}
                </span>
              </li>
            ))}
          </ul>

          <div className="hero__actions">
            <a
              className="btn btn--primary"
              href={links.repo}
              target="_blank"
              rel="noopener noreferrer"
            >
              <Icon name="github" size={15} />
              {copy.hero.source}
            </a>
            <a
              className="btn btn--secondary"
              href={links.architecture}
              target="_blank"
              rel="noopener noreferrer"
            >
              {copy.hero.architecture}
              <Icon name="arrow" size={14} />
            </a>
          </div>

          <div className="hero__runs">
            <span>{copy.hero.runsOn}</span>
            <ul>
              {copy.platforms.items.map((platform) => (
                <li key={platform.id}>{platform.name}</li>
              ))}
            </ul>
          </div>

          <div className="hero__command-wrap">
            <span className="hero__command-label">{copy.hero.commandLabel}</span>
            <div className="hero__command">
              <span className="hero__prompt" aria-hidden="true">
                $
              </span>
              <code>{copy.hero.command}</code>
              <CopyButton
                text={copy.hero.command}
                copy={copy}
                label={copy.hero.commandCopy}
                className="hero__copy-command"
              />
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
