export const defaultSpeedTarget =
  "https://speed.cloudflare.com/__down?bytes=10000000";

/** Preserve the size selector fix: Cloudflare's bytes parameter must stay < 100M. */
export function speedTarget(target: string, mb: number) {
  return target.includes("speed.cloudflare.com")
    ? `https://speed.cloudflare.com/__down?bytes=${mb >= 100 ? 99999999 : mb * 1048576}`
    : target;
}
