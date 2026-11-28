import request from "@/utils/request";

export const getRuleSources = () =>
  request({ url: "/api/v1/rules/sources", method: "get" });

export const createRuleSource = (data: {
  name: string;
  repo: string;
  branch?: string;
  pathPrefix?: string;
  platform?: string;
  behavior?: string;
}) =>
  request({
    url: "/api/v1/rules/sources",
    method: "post",
    data,
    timeout: 40000,
  });

export const deleteRuleSource = (key: string) =>
  request({
    url: `/api/v1/rules/sources/${encodeURIComponent(key)}`,
    method: "delete",
  });

export const getRuleCatalog = (params: any) =>
  request({
    url: "/api/v1/rules/catalog",
    method: "get",
    params,
  });

export const getRulePreview = (id: string) =>
  request({
    url: "/api/v1/rules/preview",
    method: "get",
    params: { id },
  });
export const getRuleTemplateGroups = (template: string) =>
  request({
    url: "/api/v1/rules/template-groups",
    method: "get",
    params: { template },
  });

export function syncRuleCatalog(source = "") {
  const data = new FormData();
  if (source) data.append("source", source);
  return request({
    url: "/api/v1/rules/sync",
    method: "post",
    data,
    headers: { "Content-Type": "multipart/form-data" },
  });
}

export const applyRulesToTemplate = (data: any) =>
  request({
    url: "/api/v1/rules/import",
    method: "post",
    data,
  });
