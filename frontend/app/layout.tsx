import type { Metadata } from "next";
import "./page.css";

export const metadata: Metadata = {
  title: "供应商寻源台",
  description: "AI 驱动的供应商寻源与现场审核工作台",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="zh-CN"><body>{children}</body></html>;
}
