/**
 * 时间展示与解析统一固定为东八区（UTC+8）。
 *
 * 面板按服务器本地（+08）分桶记录每日流量，看板又按“今天”生成 30 天日期轴。
 * 如果跟着浏览器时区走，身处其他时区的用户会看到日期整体错一天，时间也与
 * 服务器日志对不上。东八区没有夏令时，因此可以用固定偏移换算，既不依赖
 * 浏览器时区，也便于测试。
 */
export const DISPLAY_TIME_ZONE = "Asia/Shanghai";
export const DISPLAY_TIME_ZONE_LABEL = "东八区（UTC+8）";
const OFFSET_MS = 8 * 3600 * 1000;
const HOUR_MS = 3600 * 1000;
const DAY_MS = 24 * HOUR_MS;

/** 无时区标记的“墙钟时间”，按东八区解释。 */
const WALL_CLOCK =
  /^(\d{4})-(\d{2})-(\d{2})[T ](\d{2}):(\d{2})(?::(\d{2})(?:\.(\d{1,3}))?)?$/;

/** 把后端时间、ISO 字符串或东八区墙钟时间统一转成毫秒时间戳。 */
export function parseInstant(
  value: string | number | Date | null | undefined
): number | null {
  if (value === null || value === undefined || value === "") return null;
  if (value instanceof Date)
    return Number.isFinite(value.getTime()) ? value.getTime() : null;
  if (typeof value === "number") return Number.isFinite(value) ? value : null;
  if (typeof value !== "string") return null;
  const text = value.trim();
  if (!text) return null;
  const wall = WALL_CLOCK.exec(text);
  if (wall) {
    const [, year, month, day, hour, minute, second = "0", ms = "0"] = wall;
    return (
      Date.UTC(
        Number(year),
        Number(month) - 1,
        Number(day),
        Number(hour),
        Number(minute),
        Number(second),
        Number(ms.padEnd(3, "0"))
      ) - OFFSET_MS
    );
  }
  const epoch = new Date(text).getTime();
  return Number.isFinite(epoch) ? epoch : null;
}

const pad = (value: number) => String(value).padStart(2, "0");
const shifted = (epoch: number) => new Date(epoch + OFFSET_MS);

/** 东八区的日期部分：YYYY-MM-DD。 */
export function formatDay(value: string | number | Date | null | undefined) {
  const epoch = parseInstant(value);
  if (epoch === null) return "";
  const date = shifted(epoch);
  return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())}`;
}

/** 东八区的时刻部分：HH:mm（可选秒）。 */
export function formatClock(
  value: string | number | Date | null | undefined,
  withSeconds = false
) {
  const epoch = parseInstant(value);
  if (epoch === null) return "";
  const date = shifted(epoch);
  const clock = `${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())}`;
  return withSeconds ? `${clock}:${pad(date.getUTCSeconds())}` : clock;
}

/** 东八区的完整时间：YYYY-MM-DD HH:mm，不跟浏览器时区漂移。 */
export function formatDateTime(
  value: string | number | Date | null | undefined
) {
  const epoch = parseInstant(value);
  if (epoch === null) return "";
  return `${formatDay(epoch)} ${formatClock(epoch)}`;
}

/** 东八区的 MM-DD HH:mm，用于紧凑坐标轴。 */
export function formatDayClock(value: string | number | Date | null | undefined) {
  const epoch = parseInstant(value);
  if (epoch === null) return "";
  return `${formatDay(epoch).slice(5)} ${formatClock(epoch)}`;
}

/** `datetime-local` 输入框使用的东八区墙钟值：YYYY-MM-DDTHH:mm。 */
export function toInputValue(value: string | number | Date | null | undefined) {
  const epoch = parseInstant(value);
  if (epoch === null) return "";
  return `${formatDay(epoch)}T${formatClock(epoch)}`;
}

/** 把 `datetime-local` 的东八区墙钟值转回毫秒时间戳。 */
export const fromInputValue = parseInstant;

/** 东八区当天零点的时间戳。 */
export function dayStart(value: string | number | Date = Date.now()) {
  const epoch = parseInstant(value);
  if (epoch === null) return NaN;
  const date = shifted(epoch);
  return (
    Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate()) -
    OFFSET_MS
  );
}

/** 最近 N 天（含今天）的东八区日期键，从早到晚排列。 */
export function recentDayKeys(days: number, now: string | number | Date = Date.now()) {
  const base = dayStart(now);
  if (!Number.isFinite(base) || days <= 0) return [];
  return Array.from({ length: days }, (_, index) =>
    formatDay(base - (days - 1 - index) * DAY_MS)
  );
}

/** 把时刻截断到所在的 UTC 整点，供小时桶查询使用。 */
export function hourFloor(value: string | number | Date = Date.now()) {
  const epoch = parseInstant(value);
  if (epoch === null) return NaN;
  return Math.floor(epoch / HOUR_MS) * HOUR_MS;
}

/**
 * 默认查询窗口：最近 24 个 UTC 小时桶，含正在进行的小时。
 * 刚开启采集的用户立刻能看到数据，不必等到整点之后。
 */
export function recentHourWindow(now: string | number | Date = Date.now()) {
  const end = hourFloor(now) + HOUR_MS;
  return { start: end - 24 * HOUR_MS, end };
}
