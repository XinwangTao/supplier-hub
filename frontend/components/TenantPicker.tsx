"use client";

import { useCallback, useEffect, useState } from "react";
import { ensureTenant, fetchTenants, type Tenant } from "@/lib/api";
import { readTenant, subscribeTenant, writeTenant } from "@/lib/tenant";

export type { Tenant };

// 选择"当前是哪家企业在操作"。
//
// 寻源池是公开的，但"引入 SRM"这个动作必须落在某一个采购方名下 ——
// 平台是多租户的，企业选错，订单和评分就记到别人家去了。
// 顶栏与首页共用这一个控件，避免两处状态各写一份。
export default function TenantPicker({ compact = false }: { compact?: boolean }) {
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [tenant, setTenant] = useState("");
  const [creating, setCreating] = useState(false);
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(async () => {
    try {
      const list = await fetchTenants();
      setTenants(list);
      const stored = readTenant();
      const next = stored && list.some((t) => t.id === stored) ? stored : list[0]?.id || "";
      setTenant(next);
      if (next !== stored) writeTenant(next);
    } catch {
      setTenants([]);
    }
  }, []);

  useEffect(() => {
    void refresh();
    return subscribeTenant((id) => setTenant(id));
  }, [refresh]);

  const onCreate = async () => {
    const name = draft.trim();
    if (!name) return;
    setBusy(true);
    try {
      const t = await ensureTenant({ name, industry: "未填写" });
      writeTenant(t.id);
      setDraft("");
      setCreating(false);
      await refresh();
    } finally {
      setBusy(false);
    }
  };

  if (creating) {
    return (
      <span className="tenantCreate">
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          placeholder="企业名称"
          aria-label="企业名称"
          onKeyDown={(e) => {
            if (e.key === "Enter") void onCreate();
          }}
        />
        <button className="btn primary sm" disabled={busy} onClick={() => void onCreate()}>
          {busy ? "建档中…" : "建档"}
        </button>
        <button className="btn ghost sm" onClick={() => setCreating(false)}>
          取消
        </button>
      </span>
    );
  }

  return (
    <span className={`tenantPicker ${compact ? "compact" : ""}`}>
      {!compact && <span className="tenantLabel">当前企业</span>}
      <select
        className="select"
        value={tenant}
        onChange={(e) => {
          writeTenant(e.target.value);
          setTenant(e.target.value);
        }}
        aria-label="当前企业"
      >
        {tenants.length === 0 && <option value="">（尚未建档）</option>}
        {tenants.map((t) => (
          <option key={t.id} value={t.id}>
            {t.name}
          </option>
        ))}
      </select>
      <button className="btn ghost sm" onClick={() => setCreating(true)} title="登记一家新的采购方企业">
        ＋ 企业
      </button>
    </span>
  );
}
