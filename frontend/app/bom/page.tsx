"use client";

import { useEffect, useMemo, useState } from "react";
import NavBar from "@/components/NavBar";
import { ProductSupplyPanel } from "@/components/BomSupply";
import { fetchBomDetail, fetchBomIndex, type BomDetail, type BomIndex, type BomProductSummary } from "@/lib/api";

const CAP_LABEL: Record<string, string> = {
  cnc: "CNC", mold: "模具", injection: "注塑", diecasting: "压铸", stamping: "冲压",
  mim: "MIM", anodizing: "阳极", coating: "表面处理", glass: "玻璃", silicone: "硅胶",
  band: "表带", fastener: "紧固件", laser: "激光", welding: "焊接", pcb: "PCB",
  pcba: "PCBA", fpc: "FPC", display: "显示", camera: "摄像头", sensor: "传感器",
  battery: "电池", charger: "电源", motor: "电机", acoustics: "声学", connector: "连接器",
  magnet: "磁材", lens: "镜头", antenna: "天线", heatsink: "散热", assembly: "组装",
  testing: "检测", packaging: "包装",
};

// 两个视图：部件结构回答"要什么料"，供应源回答"找谁买"。
// 分开是因为两者看的人不同——研发看结构，采购看供应。
type View = "structure" | "supply";

export default function BomPage() {
  const [index, setIndex] = useState<BomIndex | null>(null);
  const [active, setActive] = useState("smartwatch");
  const [detail, setDetail] = useState<BomDetail | null>(null);
  const [filter, setFilter] = useState("");
  const [error, setError] = useState("");
  const [view, setView] = useState<View>("structure");

  useEffect(() => {
    fetchBomIndex()
      .then((data) => {
        setIndex(data);
        const qs = new URLSearchParams(window.location.search);
        const preset = qs.get("p");
        const code = preset && data.items.some((p) => p.code === preset) ? preset : data.items[0]?.code || "smartwatch";
        setActive(code);
        // 支持直接链到供应源视图：采购关心的不是结构，是哪几类料会卡脖子。
        if (qs.get("v") === "supply") setView("supply");
      })
      .catch((err) => setError(err instanceof Error ? err.message : "加载失败"));
  }, []);

  useEffect(() => {
    if (!active) return;
    fetchBomDetail(active)
      .then(setDetail)
      .catch(() => setDetail(null));
  }, [active]);

  const sectors = useMemo<{ name: string; items: BomProductSummary[] }[]>(() => {
    if (!index) return [];
    // 板块顺序固定：从消费电子到工业装备，按"离消费者由近到远"排，
    // 否则 map 遍历顺序不稳定，每次刷新列表都会跳。
    const order = ["消费电子", "家电", "智能家居", "出行", "工业装备"];
    return Object.entries(index.sectors)
      .sort((a, b) => {
        const ia = order.indexOf(a[0]);
        const ib = order.indexOf(b[0]);
        return (ia < 0 ? 99 : ia) - (ib < 0 ? 99 : ib);
      })
      .map(([name, names]) => ({
        name,
        items: index.items.filter((p) => names.includes(p.name)),
      }));
  }, [index]);

  const groups = useMemo(() => {
    if (!detail) return [];
    if (!filter) return detail.groups;
    return detail.groups
      .map((g) => ({ ...g, parts: g.parts.filter((p) => p.category === filter) }))
      .filter((g) => g.parts.length > 0);
  }, [detail, filter]);

  const criticalCount = detail?.groups.reduce((sum, g) => sum + g.parts.filter((p) => p.critical).length, 0) || 0;

  return (
    <div className="console">
      <NavBar />
      <main className="consoleBody bomBody">
        <div className="pageHead">
          <div>
            <h1>产品 BOM 知识库</h1>
            <p>
              把产品拆到部件级，再归到大类与小类。小类是跨产品复用的口径 —— 它决定了"这一类料谁能供"能不能聚合起来。
              部件结构整理自公开拆解资料与公开 BOM 目录，用于寻源下钻，不代表任何具体机型的官方 BOM。
            </p>
          </div>
          <div className="viewTabs">
            <button className={view === "structure" ? "on" : ""} onClick={() => setView("structure")}>
              部件结构
            </button>
            <button className={view === "supply" ? "on" : ""} onClick={() => setView("supply")}>
              供应源
            </button>
          </div>
        </div>

        {error && <div className="errBanner">{error}</div>}

        {index && view === "structure" && (
          <section className="facetStrip">
            {index.facets.slice(0, 10).map((f) => (
              <button
                key={f.category.code}
                className={`facetChip ${filter === f.category.code ? "on" : ""}`}
                onClick={() => setFilter(filter === f.category.code ? "" : f.category.code)}
                title={f.products.join("、")}
              >
                <b>{f.category.name}</b>
                <span>{f.parts} 件</span>
              </button>
            ))}
            {filter && (
              <button className="facetChip clear" onClick={() => setFilter("")}>
                清除筛选 ×
              </button>
            )}
          </section>
        )}

        <div className="bomLayout">
          <aside className="bomSide">
            {sectors.map((sector) => (
              <div key={sector.name} className="bomSector">
                <h3>{sector.name}</h3>
                {sector.items.map((p) => (
                  <button
                    key={p.code}
                    className={`bomItem ${active === p.code ? "on" : ""}`}
                    onClick={() => {
                      setActive(p.code);
                      setFilter("");
                    }}
                  >
                    <b>{p.name}</b>
                    <small>
                      {p.groups} 大类 · {p.part_count} 部件
                    </small>
                  </button>
                ))}
              </div>
            ))}
          </aside>

          <section className="bomMain">
            {!detail && <div className="emptyBlock">加载中…</div>}
            {detail && view === "supply" && (
              <ProductSupplyPanel productCode={active} productName={detail.product.name} />
            )}
            {detail && view === "structure" && (
              <>
                <div className="bomDetailHead">
                  <div>
                    <h2>{detail.product.name}</h2>
                    <p>{detail.product.summary}</p>
                  </div>
                  <div className="bomStats">
                    <span>
                      <b>{detail.groups.length}</b> 大类
                    </span>
                    <span>
                      <b>{detail.total_parts}</b> 部件
                    </span>
                    <span>
                      <b>{criticalCount}</b> 关键件
                    </span>
                    {detail.product.top_parts > 0 && (
                      <span className="muted">
                        公开拆解统计 <b>{detail.product.top_parts}</b> 个零件
                      </span>
                    )}
                  </div>
                </div>

                <div className="capTags">
                  {detail.product.capabilities.map((cap) => (
                    <span key={cap} className="capTag">
                      {CAP_LABEL[cap] || cap}
                    </span>
                  ))}
                </div>

                <div className="bomGroups">
                  {groups.map((group) => (
                    <div key={group.code} className={`bomGroup ${group.critical ? "critical" : ""}`}>
                      <div className="bomGroupHead">
                        <div>
                          <b>{group.name}</b>
                          <small>{group.role || "—"}</small>
                        </div>
                        <span className={`groupTag ${group.critical ? "crit" : ""}`}>
                          {group.critical ? "关键路径" : `${group.parts.length} 部件`}
                        </span>
                      </div>
                      <table className="bomTable">
                        <tbody>
                          {group.parts.map((part) => (
                            <tr key={`${group.code}-${part.code}`}>
                              <td className="bomPartName">
                                {part.name}
                                {part.critical && <em className="critDot" title="关键件：缺供会直接停线">关键</em>}
                              </td>
                              <td className="bomCat">{part.category_name}</td>
                              <td className="bomQty mono">{part.qty}</td>
                              <td className="bomCap">
                                {part.capability ? (
                                  <span className="capMono">{CAP_LABEL[part.capability] || part.capability}</span>
                                ) : (
                                  "—"
                                )}
                              </td>
                              <td className="bomNote">{part.note}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  ))}
                </div>
              </>
            )}
          </section>
        </div>
      </main>
    </div>
  );
}
