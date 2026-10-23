/**
 * 绝对时间统一按东八区（UTC+8）显示，不跟浏览器时区漂移，
 * 便于和服务器日志、每日分桶对齐。
 *
 * 保留这个入口，历史调用方无需改动。
 */
export { formatDateTime } from "./datetime";
