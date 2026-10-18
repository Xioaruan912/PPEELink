/** 登录账户信息 */
export interface UserInfo {
  userId?: number;
  username?: string;
  nickname?: string;
  roles: string[];
  perms: string[];
  mustChangePassword?: boolean;
}

export type UserRole = "admin" | "user";

/** 用户管理列表项 */
export interface ManagedUser {
  id: number;
  username: string;
  nickname: string;
  role: UserRole;
  remark: string;
  disabled: boolean;
  mustChangePassword: boolean;
  subscriptionIds: number[];
}

export interface ManagedUserForm {
  id?: number;
  username: string;
  password?: string;
  nickname: string;
  role: UserRole;
  remark: string;
  disabled: boolean;
  subscriptionIds: number[];
}

export interface TrafficSnapshot {
  reported: boolean;
  upload: number;
  download: number;
  total: number;
  expire: string | null;
}

/** 我的订阅 */
export interface MySubscription {
  id: number;
  name: string;
  expiresAt: string | null;
  links: Record<string, string>;
  imports: Record<string, string>;
  traffic: TrafficSnapshot;
}
