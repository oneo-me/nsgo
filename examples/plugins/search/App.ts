// A plugin that gives the host functions rather than a view: what its
// entry exports, which the host calls with tsx.Plugin.Call.

const words = ["native", "signed", "sandboxed", "plugins"];

export function search(query: string): string[] {
  return words.filter((w) => w.includes(query.toLowerCase()));
}
