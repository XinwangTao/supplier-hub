"use client";

// 五维档案展示面板:只陈述事实,缺失显示「—」。
// 没录过就明说"暂无巡厂记录",绝不渲染一个看似完整的空档案。

import { useEffect, useState } from "react";
import { fetchFourM1E, fetchFourM1EHistory, type FourM1E } from "../lib/api";

const DASH = "—";

function fmt(v: number | undefined, unit = ""): string {
  if (!v) return DASH;
  return `${v.toLocaleString("zh-CN")}${unit}`;
}
function txt(v: string | undefined): string {
  return v && v.trim() ? v : DASH;
}

// 历史快照一行概览:时间 + 录入人 + 少量核心字段,点开完整记录靠 API(history 可回查)。
function HistoryRow({ rec }: { rec: FourM1E }) {
  const deptTotal = rec.departments?.reduce((sum, d) => sum + (d.count || 0), 0) ?? 0;
  return (
    <li style={{ margin: "4px 0", fontSize: 12, color: "#47655c" }}>
      <b style={{ color: "#1b3b32" }}>{rec.recorded_at?.slice(0, 16).replace("T", " ") || DASH}</b>
      {" · "}{txt(rec.recorded_by)}
      {" · 员工 "}{fmt(rec.employee_total)}
      {" · 厂房 "}{fmt(rec.factory_area, "㎡")}
      {" · 产线 "}{fmt(rec.production_lines)}
      {deptTotal > 0 && ` · 部门人数合计 ${deptTotal}`}
      {rec.certifications?.length > 0 && ` · 认证 ${rec.certifications.length} 张`}
    </li>
  );
}

export default function FourM1EPanel({ supplierId }: { supplierId: string }) {
  const [rec, setRec] = useState<FourM1E | null>(null);
  const [history, setHistory] = useState<FourM1E[]>([]);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    let alive = true;
    setLoaded(false);
    Promise.all([fetchFourM1E(supplierId), fetchFourM1EHistory(supplierId).catch(() => [])])
      .then(([env, history]) => {
        if (!alive) return;
        setRec(env.data);
        setHistory(history);
      })
      .catch(() => { if (alive) { setRec(null); setHistory([]); } })
      .finally(() => { if (alive) setLoaded(true); });
    return () => { alive = false; };
  }, [supplierId]);

  if (!loaded) return <p style={{ fontSize: 12, color: "#5a7268" }}>载入五维档案…</p>;
  if (!rec) {
    return (
      <p style={{ fontSize: 13, color: "#5a7268", margin: "6px 0" }}>
        暂无巡厂记录——五维信息(人/机/料/法/环)靠人工录入,录入后这里会按维度展示事实。
      </p>
    );
  }

  const kv: React.CSSProperties = { margin: "2px 0", fontSize: 13 };
  const k: React.CSSProperties = { color: "#5a7268", display: "inline-block", minWidth: 128 };
  const section: React.CSSProperties = { borderLeft: "3px solid #0e6253", paddingLeft: 10, marginTop: 8 };

  return (
    <div style={{ fontSize: 13 }}>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 24px" }}>
        <div style={section}>
          <b style={{ color: "#0e6253" }}>人</b>
          <p style={kv}><span style={k}>员工总数(现场)</span>{fmt(rec.employee_total, " 人")}</p>
          <p style={kv}><span style={k}>研发/工程人员</span>{fmt(rec.rnd_count, " 人")}</p>
          {rec.departments?.length > 0 && (
            <p style={kv}><span style={k}>部门分布</span>
              {rec.departments.map((d) => `${d.name} ${d.count}人`).join(" · ")}
            </p>
          )}
          <p style={kv}><span style={k}>关键人员</span>{txt(rec.key_personnel)}</p>
        </div>

        <div style={section}>
          <b style={{ color: "#0e6253" }}>机</b>
          <p style={kv}><span style={k}>厂房面积</span>{fmt(rec.factory_area, " ㎡")}</p>
          <p style={kv}><span style={k}>产线数</span>{fmt(rec.production_lines)}</p>
          <p style={kv}><span style={k}>自动化率</span>{rec.automation_rate ? `${rec.automation_rate}%` : DASH}</p>
          <p style={kv}><span style={k}>产能利用率</span>{rec.capacity_utilization ? `${rec.capacity_utilization}%` : DASH}</p>
          {rec.machines?.length > 0 && (
            <p style={kv}><span style={k}>关键设备</span>
              {rec.machines.map((m) => `${m.name}${m.model ? `(${m.model})` : ""}×${m.qty}${m.age_year ? `/${m.age_year}年` : ""}`).join(" · ")}
            </p>
          )}
        </div>

        <div style={section}>
          <b style={{ color: "#0e6253" }}>料</b>
          <p style={kv}><span style={k}>主要原材料</span>{txt(rec.main_materials)}</p>
          <p style={kv}><span style={k}>双源情况</span>{txt(rec.dual_sourcing)}</p>
          <p style={kv}><span style={k}>来料检验(IQC)</span>{txt(rec.iqc_method)}</p>
          <p style={kv}><span style={k}>材料合规</span>{txt(rec.material_compliance)}</p>
        </div>

        <div style={section}>
          <b style={{ color: "#0e6253" }}>法</b>
          {rec.certifications?.length > 0 && (
            <p style={kv}><span style={k}>体系认证</span>{rec.certifications.filter(Boolean).join(" · ") || DASH}</p>
          )}
          <p style={kv}><span style={k}>设计能力</span>{txt(rec.design_capability)}</p>
          <p style={kv}><span style={k}>检测能力</span>{txt(rec.inspection_capability)}</p>
          <p style={kv}><span style={k}>工艺文件</span>{txt(rec.process_docs)}</p>
          <p style={kv}><span style={k}>变更管理</span>{txt(rec.change_management)}</p>
        </div>

        <div style={section}>
          <b style={{ color: "#0e6253" }}>环</b>
          <p style={kv}><span style={k}>环境体系</span>{txt(rec.env_certification)}</p>
          <p style={kv}><span style={k}>环保处罚</span>{txt(rec.env_penalty)}</p>
          <p style={kv}><span style={k}>能耗与三废</span>{txt(rec.waste_treatment)}</p>
          <p style={kv}><span style={k}>厂区所在地</span>{txt(rec.location)}</p>
        </div>

        <div style={section}>
          <b style={{ color: "#5a7268" }}>留痕</b>
          <p style={kv}><span style={k}>录入人</span>{txt(rec.recorded_by)}</p>
          <p style={kv}><span style={k}>录入时间</span>{rec.recorded_at ? rec.recorded_at.slice(0, 10) : DASH}</p>
          <p style={kv}><span style={k}>历史快照</span>{history.length} 条(修正不覆盖,全部可回查)</p>
          {history.length > 0 && (
            <details style={{ marginTop: 4 }}>
              <summary style={{ cursor: "pointer", fontSize: 12, color: "#0e6253" }}>展开录入历史</summary>
              <ul style={{ paddingLeft: 16, margin: "4px 0 0" }}>
                {history.slice().reverse().map((h, i) => <HistoryRow key={`${h.recorded_at}-${i}`} rec={h} />)}
              </ul>
            </details>
          )}
        </div>
      </div>
    </div>
  );
}
