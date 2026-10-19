import request from "@/utils/request";

/** 后台任务列表（规则同步等长任务的进度）。 */
export function getTasks(limit = 100) {
  return request({
    url: "/api/v1/tasks/list",
    method: "get",
    params: { limit },
  });
}
