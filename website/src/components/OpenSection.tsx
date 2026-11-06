import type { Copy } from '../copy';
import { links } from '../copy';
import { Icon } from './Icon';

export function OpenSection({ copy }: { copy: Copy }) {
  return (
    <section className="section open" id="open">
      <div className="open__grid" aria-hidden="true" />
      <div className="container open__inner">
        <div className="open__intro">
          <span className="kicker">{copy.open.kicker}</span>
          <h2>{copy.open.title}</h2>
          <p>{copy.open.body}</p>
          <div className="open__links">
            {copy.open.links.map((link) => (
              <a key={link.href} href={link.href} target="_blank" rel="noopener noreferrer">
                {link.label}
                <Icon name="external" size={11} />
              </a>
            ))}
          </div>
        </div>

        <div className="open__cta">
          <h3>{copy.open.ctaTitle}</h3>
          <p>{copy.open.ctaBody}</p>
          <div className="open__actions">
            <a
              className="btn btn--primary"
              href={links.gallery}
              target="_blank"
              rel="noopener noreferrer"
            >
              <Icon name="arrow" size={14} />
              {copy.open.view}
            </a>
            <a
              className="btn btn--secondary"
              href={links.repo}
              target="_blank"
              rel="noopener noreferrer"
            >
              <Icon name="github" size={15} />
              {copy.open.github}
            </a>
          </div>
          <p className="open__note">{copy.open.note}</p>
          <p className="open__attribution">
            {copy.open.attribution}{' '}
            <a href={links.nshSite} target="_blank" rel="noopener noreferrer">
              nsh.dev
            </a>
          </p>
        </div>
      </div>
    </section>
  );
}
