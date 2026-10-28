import type { Metadata } from "next";
import "./page.css";
import "./console.css";

export const metadata: Metadata = {
  title: "供应链网络 · Supplier Mesh",
  description: "以产品为中心的供应链网络：产品 → 模块 → 工序 → 供应商，自动编织关联网",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="zh-CN"><body>{children}</body></html>;
}
