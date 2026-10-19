import request from "@/utils/request";

// 通过表单构建 clash 模板并保存
export function BuildClashTemplate(data: any) {
  return request({
    url: "/api/v1/template/build",
    method: "post",
    data,
    headers: {
      "Content-Type": "application/json",
    },
  });
}

// 获取默认 mihomo 配置（用于预填表单）
export function GetDefaultTemplate() {
  return request({
    url: "/api/v1/template/default",
    method: "get",
  });
}

/** 把已保存的 mihomo 模板解析成构建器表单。 */
export function ParseTemplateForBuilder(filename: string, text: string) {
  return request({
    url: "/api/v1/template/parse",
    method: "post",
    data: { filename, text },
    headers: { "Content-Type": "multipart/form-data" },
  });
}
