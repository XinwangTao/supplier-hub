"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import SiteNav from "@/components/SiteNav";
import Link from "next/link";
import NetworkGraph from "../components/NetworkGraph";
import DetailPanel from "../components/DetailPanel";
import TenantPicker from "../components/TenantPicker";
import {
  buildProductGraph,
  fetchBriefs,
  fetchDatasetMeta,
  fetchMesh,
  fetchProducts,
  fetchSupplier,
  type BriefIndex,
  type DatasetMeta,
  type Graph,
  type GraphNode,
  type ProductSummary,
  type Supplier,
} from "../lib/api";
import { readTenant, subscribeTenant } from "../lib/tenant";

const REGIONS = ["不限", "广东省", "江苏省", "浙江省", "深圳市", "东莞市", "苏州市", "宁波市", "上海市"];
const EXAMPLES = ["智能手表", "TWS 蓝牙耳机", "扫地机器人", "无人机", "智能门锁"];

export default function Home() {
  const [graph, setGraph] = useState<Graph | null>(null);
  const [products, setProducts] = useState<ProductSummary[]>([]);
  const [dataset, setDataset] = useState<DatasetMeta | null>(null);
  const [query, setQuery] = useState("我想做一个智能手表");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [selected, setSelected] = useState<GraphNode | null>(null);
  const [supplier, setSupplier] = useState<Supplier | null>(null);
  const [supplierLoading, setSupplierLoading] = useState(false);
  const [maxPerProcess, setMaxPerProcess] = useState(2);
  const [region, setRegion] = useState("不限");
  const [criticalOnly, setCriticalOnly] = useState(false);
  // 平台评级索引：按 supplier_id 合并到图上，节点与详情面板都能显示"别人用下来怎么样"
  const [tenant, setTenant] = useState("");
  const [briefs, setBriefs] = useState<Record<string, BriefIndex["items"][string]>>({});

  const abortRef = useRef<AbortController | null>(null);

  const loadBriefs = useCallback((tenantId: string) => {
    fetchBriefs(tenantId)
      .then((index) => setBriefs(index.items || {}))
      .catch(() => setBriefs({}));
  }, []);

  const runBuild = useCallback(
    async (text: string, options: { max?: number; region?: string; critical?: boolean } = {}) => {
      abortRef.current?.abort();
      const controller = new AbortController();
      abortRef.current = controller;
      setLoading(true);
      setError("");
      setSelected(null);
      setSupplier(null);
      try {
        const data = await buildProductGraph(
          text,
          {
            max_suppliers_per_process: options.max ?? maxPerProcess,
            region: (options.region ?? region) === "不限" ? "" : options.region ?? region,
            critical_only: options.critical ?? criticalOnly,
          },
          controller.signal,
        );
        setGraph(data);
      } catch (err) {
        if ((err as Error).name !== "AbortError") setError(err instanceof Error ? err.message : "网络图生成失败");
      } finally {
        setLoading(false);
      }
    },
    [maxPerProcess, region, criticalOnly],
  );

  const loadMesh = useCallback(async () => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    setLoading(true);
    setError("");
    setSelected(null);
    try {
      setGraph(await fetchMesh(controller.signal));
    } catch (err) {
      if ((err as Error).name !== "AbortError") setError("全域网加载失败，请确认后端已启动");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const presetSupplier = params.get("supplier");
    const preset = params.get("q");
    if (preset) {
      setQuery(preset);
      void runBuild(preset);
    } else {
      void loadMesh();
    }
    if (presetSupplier) {
      fetchSupplier(presetSupplier)
        .then(setSupplier)
        .catch(() => undefined);
    }
    fetchProducts()
      .then(setProducts)
      .catch(() => undefined);
    fetchDatasetMeta()
      .then(setDataset)
      .catch(() => undefined);

    // 企业切换后，寻源页上的"引入 SRM"归属要跟着变
    const initialTenant = readTenant();
    setTenant(initialTenant);
    loadBriefs(initialTenant);
    const unsubscribe = subscribeTenant((id) => {
      setTenant(id);
      loadBriefs(id);
    });
    return unsubscribe;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!selected || selected.type !== "supplier" || !selected.supplier_id) {
      setSupplier(null);
      return;
    }
    let active = true;
    setSupplierLoading(true);
    fetchSupplier(selected.supplier_id)
      .then((data) => {
        if (active) setSupplier(data);
      })
      .catch(() => undefined)
      .finally(() => {
        if (active) setSupplierLoading(false);
      });
    return () => {
      active = false;
    };
  }, [selected]);

  const isMesh = graph?.mode === "mesh";

  return (
    <main className="app">
      <header className="topbar">
        <div className="brand">
          <span className="brandMark">S</span>
          <div>
            <b>供应链网络</b>
            <small>SUPPLY MESH</small>
          </div>
        </div>
        <SiteNav active="/" />
        <div className="topMeta">
          <span className={`liveDot ${dataset && !dataset.is_real ? "warn" : ""}`} />
          {graph ? `${graph.stats.suppliers} 家供应商在网 · ${graph.stats.cities} 座城市` : "连接中…"}
          {dataset && (
            <span className={`sourceTag ${dataset.is_real ? "real" : "fake"}`}>
              {dataset.is_real ? `真实数据 · ${dataset.source}` : "非真实数据"}
            </span>
          )}
          <TenantPicker compact />
        </div>
      </header>

      <section className="commandBar">
        <form
          className="netSearch"
          onSubmit={(event) => {
            event.preventDefault();
            void runBuild(query);
          }}
        >
          <span className="netSearchIcon">⌕</span>
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="说出你想做的产品，例如：我想做一个智能手表"
            aria-label="产品需求"
          />
          <button className="primaryButton" type="submit" disabled={loading}>
            {loading ? "编织中…" : "生成网络 →"}
          </button>
        </form>
        <div className="quickChips">
          <span>产品库</span>
          {products.slice(0, 6).map((product) => (
            <button
              key={product.product}
              onClick={() => {
                setQuery(`我想做${product.product}`);
                void runBuild(product.product);
              }}
            >
              {product.product}
              <em>{product.suppliers}</em>
            </button>
          ))}
        </div>
      </section>

      {dataset && !dataset.is_real && (
        <section className="sourceBanner">
          <b>这批供应商数据不是真实企业数据</b>
          <span>{dataset.message}</span>
          <code>SUPPLIER_SOURCE=csv SUPPLIER_CSV=data/suppliers.csv</code>
        </section>
      )}

      <section className="workbench">
        <div className="canvasCol">
          <div className="canvasMeta">
            <div className="canvasTitle">
              <span className="canvasEyebrow">{isMesh ? "MESH / 全域网" : "PRODUCT / 产品发散网"}</span>
              <h1>{isMesh ? "所有供应链，在一张网上" : graph?.product || "供应链网络"}</h1>
              {!isMesh && graph && <p>{graph.summary}</p>}
              {isMesh && <p>产品入口挂在工艺能力之上，点击任一产品即可展开它的上下游链路。</p>}
            </div>
            <div className="canvasFilters">
              {/* 页内视图切换:发散网/全域网(全站页面导航在顶部,这里只切本页视图) */}
              <div className="modeSwitch">
                <button className={!isMesh ? "on" : ""} onClick={() => void runBuild(query)}>
                  产品发散网
                </button>
                <button className={isMesh ? "on" : ""} onClick={() => void loadMesh()}>
                  供应链全域网
                </button>
              </div>
              <label>
                每家工序展示
                <select
                  value={maxPerProcess}
                  onChange={(event) => {
                    const value = Number(event.target.value);
                    setMaxPerProcess(value);
                    void runBuild(query, { max: value });
                  }}
                >
                  {[1, 2, 3, 4].map((n) => (
                    <option key={n} value={n}>
                      {n} 家
                    </option>
                  ))}
                </select>
              </label>
              <label>
                区域
                <select
                  value={region}
                  onChange={(event) => {
                    setRegion(event.target.value);
                    void runBuild(query, { region: event.target.value });
                  }}
                >
                  {REGIONS.map((item) => (
                    <option key={item} value={item}>
                      {item}
                    </option>
                  ))}
                </select>
              </label>
              <button
                className={criticalOnly ? "toggle on" : "toggle"}
                onClick={() => {
                  setCriticalOnly(!criticalOnly);
                  void runBuild(query, { critical: !criticalOnly });
                }}
              >
                仅关键工序
              </button>
            </div>
          </div>

          <div className="canvasBody">
            {error && (
              <div className="errorBanner">
                {error}
                <button onClick={() => void loadMesh()}>重试</button>
              </div>
            )}
            {graph && <NetworkGraph graph={graph} selectedId={selected?.id || null} onSelect={setSelected} />}
            {dataset && dataset.count === 0 && (
              <div className="emptySource">
                <h3>还没有真实供应商数据</h3>
                <p>
                  系统不会用生成数据充数。接入任一种真实数据源后，这张网才会长出供应商节点：
                </p>
                <ol>
                  <li>
                    从企查查 / 天眼查导出企业清单（CSV），放到 <code>backend/data/suppliers.csv</code>，启动时带
                    <code>SUPPLIER_SOURCE=csv SUPPLIER_CSV=data/suppliers.csv</code>
                  </li>
                  <li>
                    或配置企查查开放平台密钥 <code>QCC_KEY</code> / <code>QCC_SECRET_KEY</code>，用企业名单文件
                    <code>SUPPLIER_NAMES</code> 批量拉取
                  </li>
                </ol>
                <p className="emptyHint">表头用「企业名称 / 统一社会信用代码 / 法定代表人 / 注册资本 / 经营范围」等常见列名即可，系统会自动对齐。</p>
              </div>
            )}
            {loading && <div className="canvasLoading">正在解析产品结构并匹配供应商…</div>}
            {graph && !isMesh && (
              <button className="backToMesh" onClick={() => void loadMesh()}>
                ← 回到全域网
              </button>
            )}
          </div>
        </div>

        {graph ? (
          <DetailPanel
            graph={graph}
            selected={selected}
            supplier={supplier}
            supplierLoading={supplierLoading}
            onSelectNode={setSelected}
            onExpandProduct={(name) => {
              setQuery(`我想做${name}`);
              void runBuild(name);
            }}
            onClose={() => {
              setSelected(null);
              setSupplier(null);
            }}
            tenant={tenant}
            briefs={briefs}
            onBriefChanged={() => loadBriefs(tenant)}
          />
        ) : (
          <aside className="panel">
            <div className="panelLoading">加载中…</div>
          </aside>
        )}
      </section>
    </main>
  );
}
