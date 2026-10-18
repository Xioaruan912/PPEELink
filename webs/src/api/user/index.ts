import request from "@/utils/request";
import { AxiosPromise } from "axios";
import type {
  ManagedUser,
  ManagedUserForm,
  MySubscription,
  UserInfo,
} from "./types";

/** 当前账户信息（含角色与是否必须修改密码）。 */
export function getUserInfoApi(): AxiosPromise<UserInfo> {
  return request({ url: "/api/v1/users/me", method: "get" });
}

/** 修改自己的用户名与密码。 */
export function updateUserPassword(data: {
  username: string;
  password: string;
}) {
  const params = new URLSearchParams();
  params.append("username", data.username);
  params.append("password", data.password);
  return request({
    url: "/api/v1/users/update",
    method: "post",
    data: params,
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
  });
}

/** 管理员：用户列表。 */
export function listUsers(): AxiosPromise<{ list: ManagedUser[] }> {
  return request({ url: "/api/v1/users/page", method: "get" });
}

/** 管理员：新建用户。 */
export function createUser(data: ManagedUserForm) {
  return request({ url: "/api/v1/users/create", method: "post", data });
}

/** 管理员：修改用户；填写 password 即重置密码。 */
export function editUser(data: ManagedUserForm) {
  return request({ url: "/api/v1/users/edit", method: "post", data });
}

/** 管理员：删除用户。 */
export function deleteUser(id: number) {
  return request({
    url: "/api/v1/users/delete",
    method: "delete",
    params: { id },
  });
}

/** 分配给当前账户的订阅（管理员为全部）。 */
export function getMySubscriptions(): AxiosPromise<MySubscription[]> {
  return request({ url: "/api/v1/my/subscriptions", method: "get" });
}
