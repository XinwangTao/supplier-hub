"use client";

// 全站唯一导航:六个页面入口,任何页面的头部都渲染同一份(current 高亮)。
// 首页的「产品发散网/供应链全域网」是页内视图切换,不属于全站导航,不放在这里。
import Link from "next/link";

const SITE_LINKS = [
  { href: "/", label: "供应链网络" },
  { href: "/search", label: "寻源搜索" },
  { href: "/srm", label: "供应商管理" },
  { href: "/ranking", label: "平台评级" },
  { href: "/bom", label: "产品 BOM" },
  { href: "/policy", label: "数据规则" },
];

export default function SiteNav({ active, className = "navTabs" }: { active: string; className?: string }) {
  const isActive = (href: string) => (href === "/" ? active === "/" : active.startsWith(href));
  return (
    <nav className={className}>
      {SITE_LINKS.map((link) => (
        <Link key={link.href} href={link.href} className={isActive(link.href) ? "active" : ""}>
          {link.label}
        </Link>
      ))}
    </nav>
  );
}
