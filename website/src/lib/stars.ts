// The GitHub stars of the Showcases and Components pages' entries, read
// from GitHub's API when the site builds. An entry whose `source` is a
// repository's root on GitHub shows its stars; one that links a folder of
// a repository, such as an example in NSGo's, shows none, as the stars
// would be the whole repository's.
//
// GitHub answers 60 requests an hour without a token, so the build sends
// GITHUB_TOKEN when it is set. A failed request leaves the entry without
// stars and never fails the build.

const repository = /^https:\/\/github\.com\/([\w.-]+)\/([\w.-]+?)(?:\.git)?\/?$/;

const counts = new Map<string, Promise<number | undefined>>();

/** withStars adds each entry's stars, or none where GitHub did not say. */
export async function withStars<T extends { source?: string }>(
  entries: T[],
): Promise<(T & { stars?: number })[]> {
  const stars = await Promise.all(entries.map((entry) => starsOf(entry.source)));
  return entries.map((entry, index) => ({ ...entry, stars: stars[index] }));
}

function starsOf(source: string | undefined): Promise<number | undefined> {
  const match = source?.match(repository);
  if (!match) return Promise.resolve(undefined);
  const name = `${match[1]}/${match[2]}`.toLowerCase();
  let count = counts.get(name);
  if (count === undefined) {
    count = fetchStars(name);
    counts.set(name, count);
  }
  return count;
}

async function fetchStars(name: string): Promise<number | undefined> {
  const headers: Record<string, string> = {
    accept: 'application/vnd.github+json',
    'x-github-api-version': '2022-11-28',
  };
  const token: string | undefined = import.meta.env.GITHUB_TOKEN;
  if (token) headers.authorization = `Bearer ${token}`;
  try {
    const response = await fetch(`https://api.github.com/repos/${name}`, {
      headers,
      signal: AbortSignal.timeout(10_000),
    });
    if (!response.ok) {
      console.warn(`stars: GitHub answered ${response.status} for ${name}; showing none`);
      return undefined;
    }
    const body = (await response.json()) as { stargazers_count?: unknown };
    return typeof body.stargazers_count === 'number' ? body.stargazers_count : undefined;
  } catch (error) {
    console.warn(`stars: could not reach GitHub for ${name}; showing none: ${error}`);
    return undefined;
  }
}

/** formatStars writes a count as GitHub does, such as 6, 1.2k or 12k. */
export function formatStars(count: number): string {
  return new Intl.NumberFormat('en', {
    notation: 'compact',
    maximumFractionDigits: 1,
  })
    .format(count)
    .toLowerCase();
}
