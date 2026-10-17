/** Display absolute timestamps in the viewer's local timezone, without locale-dependent punctuation. */
export function formatDateTime(
  value: string | number | Date | null | undefined
): string {
  if (value === null || value === undefined || value === "") return "";
  // Go's log timestamps without an offset are local wall times. Use ISO separators for Safari.
  const input =
    typeof value === "string"
      ? value.replace(/^(\d{4}-\d{2}-\d{2}) (\d{2}:\d{2})/, "$1T$2")
      : value;
  const date = new Date(input);
  if (!Number.isFinite(date.getTime())) return "";
  const pad = (part: number) => String(part).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}
