// Strip leading regional-indicator flag emojis (e.g. "🇭🇰 ", "🇺🇸 ", "🇯🇵 ")
// when an SVG <CountryFlag /> is already rendered beside the node name.
const LEADING_FLAG_RE = /^(?:[\u{1F1E6}-\u{1F1FF}]{2}\s*)+/u;

export function displayNodeName(name?: string | null): string {
  if (!name) return "";
  const stripped = name.replace(LEADING_FLAG_RE, "").trim();
  return stripped || name;
}
