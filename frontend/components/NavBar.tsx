"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import TenantPicker from "./TenantPicker";

const LINKS = [
  { href: "/", label: "供应链网络", desc: "产品 → 工序 → 供应商" },
  { href: "/search", label: "寻源搜索", desc: "公开供应商池" },
  { href: "/srm", label: "供应商管理", desc: "QCDST 与 SRM 数据" },
  { href: "/ranking", label: "平台评级", desc: "跨企业综合评级" },
  { href: "/bom", label: "产品 BOM", desc: "大类 → 小类 → 部件" },
  { href: "/policy", label: "数据规则", desc: "能看到什么、为什么" },
];

export default function NavBar() {
  const pathname = usePathname() || "/";
  // 首页只在完全等于 "/" 时高亮，否则 /srm 也会把 "/" 点亮
  const active = (href: string) => (href === "/" ? pathname === "/" : pathname.startsWith(href));

  return (
    <header className="navBar">
      <Link href="/" className="navBrand">
        <span className="navMark">SM</span>
        <span>
          <b>Supplier Mesh</b>
          <em>供应链网络 · 供应商能力平台</em>
        </span>
      </Link>
      <nav className="navLinks">
        {LINKS.map((link) => (
          <Link key={link.href} href={link.href} className={`navLink ${active(link.href) ? "on" : ""}`}>
            <span>{link.label}</span>
            <small>{link.desc}</small>
          </Link>
        ))}
      </nav>
      <div className="navTenant">
        <TenantPicker />
      </div>
    </header>
  );
}
