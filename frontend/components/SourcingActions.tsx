"use client";

import Link from "next/link";
import { useState } from "react";
import { GradeBadge } from "./Qcdst";
import {
  ENGAGEMENT_STATUS,
  engageSupplier,
  setEngagementStatus,
  type SupplierBrief,
} from "@/lib/api";

// 寻源页上的两块信息：
// 1) 平台评级 —— 别人家用下来的真实表现，这是 SRM 数据反哺回来的部分；
// 2) 引入动作 —— 把这家拉进本企业的 SRM，之后才开始积累自己的数据。
// 两块必须放在一起：先看别人怎么评价，再决定要不要自己试用。

export function RatingChip({ brief, compact = false }: { brief?: SupplierBrief; compact?: boolean }) {
  if (!brief || !brief.rated) {
    return (
      <span className="ratingChip none" title={brief?.rating_summary || "尚无企业在 SRM 中合作过"}>
        <em>平台评分</em>
        <b>暂无</b>
        {!compact && <small>{brief?.rating_summary || "还没有企业留下可评级的交付记录"}</small>}
      </span>
    );
  }
  return (
    <span className={`ratingChip ${brief.demo ? "demo" : ""}`} title={brief.rating_summary}>
      <em>平台评分</em>
      <span className="chipScore">
        <GradeBadge grade={brief.grade} provisional={brief.provisional} size="sm" />
        <b>{brief.total.toFixed(1)}</b>
      </span>
      {!compact && <small>{brief.rating_summary}</small>}
    </span>
  );
}

export function EngageControl({
  supplierId,
  supplierName,
  tenant,
  brief,
  category,
  onChanged,
}: {
  supplierId: string;
  supplierName: string;
  tenant: string;
  brief?: SupplierBrief;
  // category 引入的品类上下文。从 BOM 部件引入时传小类名，
  // 之后在 SRM 里就能看出"这家当初是为哪一类料找的"。
  category?: string;
  onChanged?: (message: string) => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const run = async (fn: () => Promise<unknown>, fallback: string) => {
    setBusy(true);
    setError("");
    try {
      const res = await fn();
      const message = typeof (res as { message?: string })?.message === "string" ? (res as { message: string }).message : "";
      onChanged?.(message || fallback);
    } catch (err) {
      setError(err instanceof Error ? err.message : "操作失败");
    } finally {
      setBusy(false);
    }
  };

  if (!tenant) {
    return <span className="engageHint">先在右上角选择企业，才能引入 SRM</span>;
  }

  if (!brief?.in_srm) {
    return (
      <span className="engageBox">
        <button
          className="btn primary sm"
          disabled={busy}
          onClick={() =>
            run(
              () =>
                engageSupplier({
                  tenant_id: tenant,
                  supplier_id: supplierId,
                  source: "sourcing",
                  status: "candidate",
                  category: category || "",
                  note: category ? `从 BOM 部件小类「${category}」引入` : "从公开寻源池引入",
                }),
              `${supplierName} 已加入候选池`,
            )
          }
        >
          {busy ? "引入中…" : "引入 SRM"}
        </button>
        {error && <em className="engageErr">{error}</em>}
      </span>
    );
  }

  const status = brief.status || "candidate";
  return (
    <span className="engageBox">
      <span className={`statusChip st-${status}`}>{brief.status_label || ENGAGEMENT_STATUS[status] || status}</span>
      {status === "candidate" && (
        <button
          className="btn ghost sm"
          disabled={busy}
          onClick={() =>
            run(
              () => setEngagementStatus(tenant, supplierId, "active"),
              `${supplierName} 已转为合作中，可以开始录订单了`,
            )
          }
        >
          转合作中
        </button>
      )}
      {(status === "active" || status === "invited") && (
        <Link className="linkBtn" href={`/vendor/${encodeURIComponent(supplierId)}?tenant=${tenant}`}>
          去管理 →
        </Link>
      )}
      {status !== "terminated" && (
        <button
          className="btn ghost sm danger"
          disabled={busy}
          onClick={() => run(() => setEngagementStatus(tenant, supplierId, "terminated"), `已终止与 ${supplierName} 的合作`)}
        >
          终止
        </button>
      )}
      {status === "terminated" && (
        <button
          className="btn ghost sm"
          disabled={busy}
          onClick={() => run(() => setEngagementStatus(tenant, supplierId, "candidate"), `${supplierName} 已退回候选池`)}
        >
          退回候选
        </button>
      )}
      {error && <em className="engageErr">{error}</em>}
    </span>
  );
}
