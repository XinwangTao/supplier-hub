"use client";

import { usePathname } from "next/navigation";
import TenantPicker from "./TenantPicker";
import SiteNav from "./SiteNav";

export default function NavBar() {
  const pathname = usePathname() || "/";
  return (
    <header className="navBar">
      <a href="/" className="navBrand">
        <span className="navMark">S</span>
        <span>
          <b>供应链网络</b>
          <em>SUPPLY MESH</em>
        </span>
      </a>
      <SiteNav active={pathname} />
      <div className="navTenant">
        <TenantPicker />
      </div>
    </header>
  );
}
