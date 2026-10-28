"use client";

// 当前操作的是哪家采购方企业。
//
// 平台是多租户的：寻源页是公开池（谁都能看），SRM 里的订单、报价、评分是各企业私有的。
// 所以"这家供应商我有没有引入过"这类判断，必须先知道当前是哪个企业在操作。
// 用 localStorage + 事件广播，避免在 NavBar 与各页面之间铺一层全局状态。

const KEY = "sm.tenant";
export const TENANT_EVENT = "sm:tenant-change";

export function readTenant(): string {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem(KEY) || "";
}

export function writeTenant(id: string) {
  if (typeof window === "undefined") return;
  if (id) {
    window.localStorage.setItem(KEY, id);
  } else {
    window.localStorage.removeItem(KEY);
  }
  window.dispatchEvent(new CustomEvent<string>(TENANT_EVENT, { detail: id }));
}

export function subscribeTenant(handler: (id: string) => void): () => void {
  if (typeof window === "undefined") return () => undefined;
  const listener = (event: Event) => handler((event as CustomEvent<string>).detail || "");
  window.addEventListener(TENANT_EVENT, listener);
  return () => window.removeEventListener(TENANT_EVENT, listener);
}
