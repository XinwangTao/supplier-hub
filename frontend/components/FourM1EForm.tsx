"use client";

// 五维档案录入表单(人·机·料·法·环)。
// 数据全部来自巡厂人工录入:这里只做「事实搬运」,不做推断、不给默认值、不算分。
// 校验失败时把后端原因原样标在表单顶部,让录入人当场改,而不是静默丢弃。

import { useEffect, useState } from "react";
import {
  fetchFourM1E, saveFourM1E, EMPTY_FOUR_M1E,
  type FourM1E, type DeptHeadcount, type MachineItem,
} from "../lib/api";

const DEFAULT_DEPARTMENTS = ["品质部", "开发部", "业务部", "生产部", "高层管理"];

export default function FourM1EForm({
  supplierId, supplierName, onSaved, onClose,
}: {
  supplierId: string;
  supplierName: string;
  onSaved?: (message: string) => void;
  onClose?: () => void;
}) {
  const [form, setForm] = useState<FourM1E>(EMPTY_FOUR_M1E(supplierId));
  const [recordedBy, setRecordedBy] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [isCorrection, setIsCorrection] = useState(false); // 有历史底稿 = 修正场景

  useEffect(() => {
    let alive = true;
    setLoading(true);
    fetchFourM1E(supplierId)
      .then((env) => {
        if (!alive) return;
        // 有历史就以最新快照为底稿(修正场景);从未录入则从空白开始
        setForm(env.data ? { ...env.data, supplier_id: supplierId } : EMPTY_FOUR_M1E(supplierId));
        setRecordedBy(env.last_recorded_by || "");
        setIsCorrection(Boolean(env.data));
      })
      .catch(() => { if (alive) setForm(EMPTY_FOUR_M1E(supplierId)); })
      .finally(() => { if (alive) setLoading(false); });
    return () => { alive = false; };
  }, [supplierId]);

  const set = <K extends keyof FourM1E>(key: K, value: FourM1E[K]) =>
    setForm((prev) => ({ ...prev, [key]: value }));

  const setNum = (key: keyof FourM1E, raw: string) => {
    const n = raw === "" ? 0 : Number(raw);
    set(key, (Number.isFinite(n) ? n : 0) as never);
  };

  const updateDept = (index: number, patch: Partial<DeptHeadcount>) =>
    set("departments", form.departments.map((d, i) => (i === index ? { ...d, ...patch } : d)));
  const updateMachine = (index: number, patch: Partial<MachineItem>) =>
    set("machines", form.machines.map((m, i) => (i === index ? { ...m, ...patch } : m)));
  const updateCert = (index: number, value: string) =>
    set("certifications", form.certifications.map((c, i) => (i === index ? value : c)));

  // 前端先挡一遍明显的非法值,后端仍会再校验——两边都拦,录入人最快拿到反馈。
  function validateLocal(): string {
    for (const [label, v] of [["员工总数", form.employee_total], ["研发人员数", form.rnd_count], ["厂房面积", form.factory_area], ["产线数", form.production_lines]] as const) {
      if (v < 0) return `${label} 不能为负数`;
    }
    if (form.automation_rate < 0 || form.automation_rate > 100) return "自动化率应在 0–100 之间";
    if (form.capacity_utilization < 0 || form.capacity_utilization > 100) return "产能利用率应在 0–100 之间";
    if (form.departments.some((d) => d.count < 0)) return "部门人数不能为负数";
    if (form.machines.some((m) => m.qty < 0 || m.age_year < 0)) return "设备数量与役龄不能为负数";
    if (!recordedBy.trim()) return "请填写录入人(巡厂审核员)";
    return "";
  }

  async function submit() {
    const problem = validateLocal();
    if (problem) { setError(problem); return; }
    setBusy(true);
    setError("");
    try {
      const saved = await saveFourM1E({ ...form, recorded_by: recordedBy.trim() });
      setForm(saved);
      onSaved?.(`已保存 ${supplierName} 的五维档案(第 ${saved.recorded_at ? "新" : ""}快照)`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
    } finally {
      setBusy(false);
    }
  }

  if (loading) return <p style={{ color: "#5a7268", padding: 12 }}>载入五维档案…</p>;

  const labelStyle: React.CSSProperties = { fontSize: 12, color: "#5a7268", display: "block", marginBottom: 2 };
  const inputStyle: React.CSSProperties = { width: "100%", boxSizing: "border-box", padding: "5px 8px", border: "1px solid #c9d8d1", borderRadius: 6, fontSize: 13, background: "#fff" };
  const cellStyle: React.CSSProperties = { flex: 1, minWidth: 0 };
  const sectionTitle: React.CSSProperties = { fontSize: 13, fontWeight: 700, color: "#0e6253", margin: "14px 0 6px", borderBottom: "1px solid #d7e2dc", paddingBottom: 4 };

  return (
    <div style={{ border: "1px solid #d7e2dc", borderRadius: 10, padding: 14, background: "#fbfdfc" }}>
      <div style={{ display: "flex", alignItems: "baseline", gap: 10 }}>
        <b>4M1E 档案 · 巡厂录入</b>
        <span style={{ fontSize: 12, color: "#5a7268" }}>{supplierName}</span>
        {onClose && (
          <button onClick={onClose} style={{ marginLeft: "auto", border: "1px solid #c9d8d1", background: "#fff", borderRadius: 6, padding: "2px 10px", cursor: "pointer" }}>
            收起
          </button>
        )}
      </div>
      <p style={{ fontSize: 12, color: "#5a7268", margin: "4px 0 0" }}>
        {isCorrection
          ? "正在基于最新快照修正:改动会生成一条新快照,旧记录保留可回查。"
          : "这里存的是巡厂现场掌握的事实,不是评分。缺失就空着,档案上会显示「—」,系统不会替你推断。"}
      </p>

      {error && (
        <div style={{ margin: "8px 0", padding: "6px 10px", background: "#fdeeea", border: "1px solid #e5b8ae", borderRadius: 6, color: "#b03a2e", fontSize: 13 }}>
          {error}
        </div>
      )}

      {/* 人 */}
      <div style={sectionTitle}>人 Man</div>
      <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
        <div style={cellStyle}><label style={labelStyle}>员工总数(现场口径)</label>
          <input style={inputStyle} type="number" min={0} value={form.employee_total || ""} onChange={(e) => setNum("employee_total", e.target.value)} /></div>
        <div style={cellStyle}><label style={labelStyle}>研发/工程人员数</label>
          <input style={inputStyle} type="number" min={0} value={form.rnd_count || ""} onChange={(e) => setNum("rnd_count", e.target.value)} /></div>
        <div style={cellStyle}><label style={labelStyle}>录入人(审核员)</label>
          <input style={inputStyle} value={recordedBy} onChange={(e) => setRecordedBy(e.target.value)} placeholder="如:质量部 周工" /></div>
      </div>
      <label style={{ ...labelStyle, marginTop: 8 }}>部门分布(人数)</label>
      {form.departments.map((dept, i) => (
        <div key={i} style={{ display: "flex", gap: 8, marginBottom: 4 }}>
          <input style={{ ...inputStyle, flex: 2 }} value={dept.name} onChange={(e) => updateDept(i, { name: e.target.value })} placeholder="部门" />
          <input style={{ ...inputStyle, flex: 1 }} type="number" min={0} value={dept.count || ""} onChange={(e) => updateDept(i, { count: e.target.value === "" ? 0 : Number(e.target.value) })} placeholder="人数" />
          <button onClick={() => set("departments", form.departments.filter((_, j) => j !== i))} style={{ border: "none", background: "none", color: "#b03a2e", cursor: "pointer" }}>✕</button>
        </div>
      ))}
      <button onClick={() => set("departments", [...form.departments, { name: "", count: 0 }])} style={{ fontSize: 12, color: "#0e6253", background: "none", border: "1px dashed #7fa79a", borderRadius: 6, padding: "2px 10px", cursor: "pointer" }}>
        + 加部门(预设:{DEFAULT_DEPARTMENTS.join("/")})
      </button>
      <div style={{ marginTop: 8 }}><label style={labelStyle}>关键人员(高管/质量负责人,姓名+职务)</label>
        <input style={inputStyle} value={form.key_personnel} onChange={(e) => set("key_personnel", e.target.value)} /></div>

      {/* 机 */}
      <div style={sectionTitle}>机 Machine</div>
      <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
        <div style={cellStyle}><label style={labelStyle}>厂房面积(㎡)</label>
          <input style={inputStyle} type="number" min={0} value={form.factory_area || ""} onChange={(e) => setNum("factory_area", e.target.value)} /></div>
        <div style={cellStyle}><label style={labelStyle}>产线数</label>
          <input style={inputStyle} type="number" min={0} value={form.production_lines || ""} onChange={(e) => setNum("production_lines", e.target.value)} /></div>
        <div style={cellStyle}><label style={labelStyle}>自动化率(%)</label>
          <input style={inputStyle} type="number" min={0} max={100} value={form.automation_rate || ""} onChange={(e) => setNum("automation_rate", e.target.value)} /></div>
        <div style={cellStyle}><label style={labelStyle}>产能利用率(%)</label>
          <input style={inputStyle} type="number" min={0} max={100} value={form.capacity_utilization || ""} onChange={(e) => setNum("capacity_utilization", e.target.value)} /></div>
      </div>
      <label style={{ ...labelStyle, marginTop: 8 }}>关键设备清单(名称/型号/数量/役龄)</label>
      {form.machines.map((m, i) => (
        <div key={i} style={{ display: "flex", gap: 6, marginBottom: 4 }}>
          <input style={{ ...inputStyle, flex: 2 }} value={m.name} onChange={(e) => updateMachine(i, { name: e.target.value })} placeholder="设备名称" />
          <input style={{ ...inputStyle, flex: 2 }} value={m.model} onChange={(e) => updateMachine(i, { model: e.target.value })} placeholder="型号" />
          <input style={{ ...inputStyle, flex: 1 }} type="number" min={0} value={m.qty || ""} onChange={(e) => updateMachine(i, { qty: e.target.value === "" ? 0 : Number(e.target.value) })} placeholder="数量" />
          <input style={{ ...inputStyle, flex: 1 }} type="number" min={0} value={m.age_year || ""} onChange={(e) => updateMachine(i, { age_year: e.target.value === "" ? 0 : Number(e.target.value) })} placeholder="役龄(年)" />
          <button onClick={() => set("machines", form.machines.filter((_, j) => j !== i))} style={{ border: "none", background: "none", color: "#b03a2e", cursor: "pointer" }}>✕</button>
        </div>
      ))}
      <button onClick={() => set("machines", [...form.machines, { name: "", model: "", qty: 0, age_year: 0 }])} style={{ fontSize: 12, color: "#0e6253", background: "none", border: "1px dashed #7fa79a", borderRadius: 6, padding: "2px 10px", cursor: "pointer" }}>
        + 加设备
      </button>

      {/* 料 */}
      <div style={sectionTitle}>料 Material</div>
      <label style={labelStyle}>主要原材料及来源地</label>
      <input style={inputStyle} value={form.main_materials} onChange={(e) => set("main_materials", e.target.value)} placeholder="如:铝锭(云南)/IC(代理分销)" />
      <div style={{ display: "flex", gap: 10, marginTop: 6 }}>
        <div style={cellStyle}><label style={labelStyle}>关键物料双源情况</label>
          <input style={inputStyle} value={form.dual_sourcing} onChange={(e) => set("dual_sourcing", e.target.value)} /></div>
        <div style={cellStyle}><label style={labelStyle}>来料检验方式(IQC)</label>
          <input style={inputStyle} value={form.iqc_method} onChange={(e) => set("iqc_method", e.target.value)} /></div>
        <div style={cellStyle}><label style={labelStyle}>材料合规(RoHS/REACH)</label>
          <input style={inputStyle} value={form.material_compliance} onChange={(e) => set("material_compliance", e.target.value)} /></div>
      </div>

      {/* 法 */}
      <div style={sectionTitle}>法 Method</div>
      <label style={labelStyle}>体系认证(一行一张证书)</label>
      {form.certifications.map((cert, i) => (
        <div key={i} style={{ display: "flex", gap: 6, marginBottom: 4 }}>
          <input style={inputStyle} value={cert} onChange={(e) => updateCert(i, e.target.value)} placeholder="如:ISO9001:2015(证书号/有效期)" />
          <button onClick={() => set("certifications", form.certifications.filter((_, j) => j !== i))} style={{ border: "none", background: "none", color: "#b03a2e", cursor: "pointer" }}>✕</button>
        </div>
      ))}
      <button onClick={() => set("certifications", [...form.certifications, ""])} style={{ fontSize: 12, color: "#0e6253", background: "none", border: "1px dashed #7fa79a", borderRadius: 6, padding: "2px 10px", cursor: "pointer" }}>
        + 加认证
      </button>
      <div style={{ display: "flex", gap: 10, marginTop: 6 }}>
        <div style={cellStyle}><label style={labelStyle}>设计能力</label>
          <input style={inputStyle} value={form.design_capability} onChange={(e) => set("design_capability", e.target.value)} /></div>
        <div style={cellStyle}><label style={labelStyle}>检测能力</label>
          <input style={inputStyle} value={form.inspection_capability} onChange={(e) => set("inspection_capability", e.target.value)} /></div>
      </div>
      <div style={{ display: "flex", gap: 10, marginTop: 6 }}>
        <div style={cellStyle}><label style={labelStyle}>工艺文件完备度</label>
          <input style={inputStyle} value={form.process_docs} onChange={(e) => set("process_docs", e.target.value)} /></div>
        <div style={cellStyle}><label style={labelStyle}>变更管理</label>
          <input style={inputStyle} value={form.change_management} onChange={(e) => set("change_management", e.target.value)} /></div>
      </div>

      {/* 环 */}
      <div style={sectionTitle}>环 Environment</div>
      <div style={{ display: "flex", gap: 10 }}>
        <div style={cellStyle}><label style={labelStyle}>环境体系(ISO14001 等)</label>
          <input style={inputStyle} value={form.env_certification} onChange={(e) => set("env_certification", e.target.value)} /></div>
        <div style={cellStyle}><label style={labelStyle}>环保处罚记录</label>
          <input style={inputStyle} value={form.env_penalty} onChange={(e) => set("env_penalty", e.target.value)} /></div>
      </div>
      <div style={{ display: "flex", gap: 10, marginTop: 6 }}>
        <div style={cellStyle}><label style={labelStyle}>能耗与三废处理</label>
          <input style={inputStyle} value={form.waste_treatment} onChange={(e) => set("waste_treatment", e.target.value)} /></div>
        <div style={cellStyle}><label style={labelStyle}>厂区所在地</label>
          <input style={inputStyle} value={form.location} onChange={(e) => set("location", e.target.value)} /></div>
      </div>

      <div style={{ marginTop: 14, display: "flex", gap: 10, alignItems: "center" }}>
        <button onClick={() => void submit()} disabled={busy}
          style={{ background: "#0e6253", color: "#fff", border: "none", borderRadius: 8, padding: "7px 20px", fontSize: 14, cursor: busy ? "wait" : "pointer" }}>
          {busy ? "保存中…" : "保存快照(留痕)"}
        </button>
        <span style={{ fontSize: 12, color: "#5a7268" }}>保存即生成一条不可覆盖的历史快照</span>
      </div>
    </div>
  );
}
