export type Risk = "low" | "medium" | "high" | "critical" | "";
export type NodeType = "product" | "module" | "process" | "supplier" | "capability";

export type Filters = {
  business_keyword: string;
  province: string;
  city: string;
  min_capital: number;
  min_employees: number;
  manufacturer_only?: boolean;
  max_risk: Risk;
};

export type GraphNode = {
  id: string;
  type: NodeType;
  label: string;
  subtitle?: string;
  level: number;
  parent_id?: string;
  supplier_id?: string;
  capability?: string;
  risk?: Risk;
  score?: number;
  meta?: Record<string, string>;
};

export type GraphEdge = {
  id: string;
  source: string;
  target: string;
  relation: "bom" | "process" | "supply" | "alternative" | "same_city" | "uses" | "cooccur";
  label?: string;
  weight: number;
};

export type Insight = {
  kind: string;
  level: "info" | "warn" | "critical";
  title: string;
  detail: string;
  node_ids?: string[];
};

export type GraphStats = {
  modules: number;
  processes: number;
  suppliers: number;
  cities: number;
  high_risk: number;
  capabilities: number;
  edges: number;
};

export type Graph = {
  mode: "product" | "mesh" | "supplier";
  query: string;
  product: string;
  summary: string;
  center: string;
  nodes: GraphNode[];
  edges: GraphEdge[];
  stats: GraphStats;
  insights: Insight[];
  generated_at: string;
};

export type Shareholder = { name: string; ratio: number; type: string; subscribed: number };
export type Personnel = { name: string; title: string };
export type Certification = { name: string; issuer: string; valid_until: string; scope: string };

export type Supplier = {
  id: string;
  name: string;
  short_name?: string;
  credit_code: string;
  legal_representative: string;
  legal_rep_title: string;
  business_status: string;
  enterprise_type: string;
  registered_address: string;
  office_address: string;
  establish_date: string;
  registered_capital: number;
  paid_capital: number;
  tax_rating: string;
  import_export: boolean;
  has_branches: boolean;
  annual_revenue: number;
  invention_patents: number;
  utility_patents: number;
  shareholders: Shareholder[];
  key_personnel: Personnel[];
  last_verified_at: string;
  business_scope: string;
  main_products: string[];
  capabilities: string[];
  capability_names: string[];
  capacity_description: string;
  factory_area: number;
  employee_count: number;
  insured_count: number;
  rnd_staff: number;
  moq: number;
  lead_time_days: number;
  certifications: Certification[];
  customers: string[];
  province: string;
  city: string;
  district: string;
  is_manufacturer: boolean;
  risk_level: Risk;
  risk_tags: string[];
  contact_name: string;
  contact_phone: string;
  contact_email: string;
  website: string;
  data_source: string;
  source_url?: string;
};

export type ScoredSupplier = Supplier & { match_score: number; reasons: string[]; overall?: OverallScore };
export type SearchResult = {
  items: ScoredSupplier[];
  total: number;
  page: number;
  page_size: number;
  applied_filters: Filters;
  interpretation: string;
};

export type ChecklistItem = { title: string; description: string; priority: string };
export type Checklist = {
  supplier_id: string;
  supplier_name: string;
  summary: string;
  sections: { category: string; items: ChecklistItem[] }[];
};

export type ProductSummary = {
  product: string;
  category: string;
  summary: string;
  aliases: string[];
  modules: number;
  processes: number;
  capabilities: number;
  suppliers: number;
};

// 默认走同源 /api（由 next.config.ts 反代到后端），也可用 NEXT_PUBLIC_API_URL 直连后端。
const API = process.env.NEXT_PUBLIC_API_URL ?? "";

async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(`${API}${path}`, { signal });
  if (!response.ok) throw new Error(`请求失败（${response.status}）`);
  return (await response.json()).data as T;
}

async function post<T>(path: string, body: unknown, signal?: AbortSignal): Promise<T> {
  const response = await fetch(`${API}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal,
  });
  if (!response.ok) throw new Error(`请求失败（${response.status}）`);
  return (await response.json()).data as T;
}

export type BuildOptions = {
  max_suppliers_per_process?: number;
  region?: string;
  critical_only?: boolean;
};

export function buildProductGraph(query: string, options: BuildOptions = {}, signal?: AbortSignal) {
  return post<Graph>("/api/v1/graph/build", { query, ...options }, signal);
}

export type DatasetMeta = {
  mode: "csv" | "qcc" | "mock" | "none" | "error" | string;
  is_real: boolean;
  count: number;
  source: string;
  message: string;
  loaded_at?: string;
  skipped?: string[];
};

export function fetchDatasetMeta(signal?: AbortSignal) {
  return get<DatasetMeta>("/api/v1/meta/dataset", signal);
}

export function fetchMesh(signal?: AbortSignal) {
  return get<Graph>("/api/v1/graph/mesh", signal);
}

export function fetchSupplier(id: string, signal?: AbortSignal) {
  return get<Supplier>(`/api/v1/suppliers/${id}`, signal);
}

export function fetchSupplierGraph(id: string, signal?: AbortSignal) {
  return get<Graph>(`/api/v1/graph/supplier/${id}`, signal);
}

export function fetchProducts(signal?: AbortSignal) {
  return get<ProductSummary[]>("/api/v1/catalog/products", signal);
}

export function fetchChecklist(id: string) {
  return post<Checklist>(`/api/v1/suppliers/${id}/audit-checklist`, undefined);
}

export function searchSuppliers(query: string, filters: Filters) {
  return post<SearchResult>("/api/v1/suppliers/search", { query, filters, page: 1, page_size: 30 });
}

export const money = (value: number) => `${value.toLocaleString("zh-CN")} 万`;

// ---------------- QCDST / SRM ----------------

export type MetricScore = {
  code: string;
  name: string;
  unit: string;
  weight: number;
  score: number; // -1 = 数据不足
  raw: string;
  numerator: number;
  denominator: number;
  sample: number;
  min_sample: number;
  confidence: number;
  formula: string;
  benchmark: string;
  evidence: string[];
  auto: boolean;
};

export type DimensionScore = {
  dimension: "Q" | "C" | "D" | "S" | "T";
  name: string;
  weight: number;
  score: number;
  confidence: number;
  coverage: number;
  summary: string;
  metrics: MetricScore[];
};

export type Grade = "A+" | "A" | "B" | "C" | "D" | "NA";

export type ScorePoint = { period: string; total: number; samples: number };

export type QCDSTAssessment = {
  supplier_id: string;
  supplier_name: string;
  short_name: string;
  credit_code: string;
  tenant_id: string;
  tenant_name: string;
  total: number;
  grade: Grade;
  confidence: number;
  coverage: number;
  dimensions: DimensionScore[];
  window: string;
  order_count: number;
  batch_count: number;
  quality_events: number;
  demo: boolean;
  provisional: boolean;
  generated_at: string;
  insights: { kind: string; level: string; title: string; detail: string }[];
  history: ScorePoint[];
};

export type PlatformRating = {
  supplier_id: string;
  supplier_name: string;
  short_name: string;
  credit_code: string;
  city: string;
  capabilities: string[];
  total: number;
  grade: Grade;
  confidence: number;
  buyer_count: number;
  rated_buyers: number;
  order_count: number;
  batch_count: number;
  quality_events: number;
  spread: number;
  dimensions: { dimension: string; name: string; score: number }[];
  verified: boolean;
  demo: boolean;
  provisional: boolean;
  last_seen_at: string;
};

export type Engagement = {
  id: string;
  tenant_id: string;
  supplier_id: string;
  status: string;
  category: string;
  since: string;
  annual_spend: number;
  share_of_spend: number;
  sourcing_types: string[];
  buyer_name: string;
  supplier_contact: string;
  source?: string;
  note?: string;
};

// ---------------- 寻源 ⇄ SRM 闭环 ----------------

// 合作关系状态机：候选 → 已邀请 → 合作中 → 暂缓 / 已终止
export const ENGAGEMENT_STATUS: Record<string, string> = {
  candidate: "候选",
  invited: "已邀请",
  active: "合作中",
  paused: "暂缓",
  terminated: "已终止",
};

// 寻源页用的摘要：平台评级（反哺结果）+ 本企业私有合作状态
export type SupplierBrief = {
  supplier_id: string;
  rated: boolean;
  grade: Grade;
  total: number;
  buyer_count: number;
  rated_buyers: number;
  batch_count: number;
  order_count: number;
  spread: number;
  provisional: boolean;
  verified: boolean;
  demo: boolean;
  rating_summary: string;
  in_srm: boolean;
  status: string;
  status_label: string;
  since: string;
  my_order_count: number;
  my_batch_count: number;
};

export type BriefIndex = {
  tenant_id: string;
  tenant_name: string;
  items: Record<string, SupplierBrief>;
  meta: {
    rated: number;
    in_srm: number;
    candidates: number;
    buyers: number;
    demo: boolean;
    note: string;
  };
};

export type LoopReport = {
  suppliers_in_pool: number;
  rated_suppliers: number;
  provisional: number;
  verified: number;
  demo: number;
  tenants: number;
  total_batches: number;
  average_spread: number;
  my_suppliers: number;
  my_candidates: number;
  my_batches: number;
  stage: string;
};

// 订单级评分卡：一张订单的 QCDST 表现
export type OrderDimension = {
  dimension: "Q" | "C" | "D" | "S" | "T";
  name: string;
  weight: number;
  effective_weight: number;
  score: number; // -1 = 这笔订单判不了
  judged: boolean;
  raw: string;
  reason: string;
};

export type OrderScorecard = {
  order_id: string;
  po_no: string;
  tenant_id: string;
  part_name: string;
  category: string;
  qty: number;
  amount: number;
  issued_at: string;
  closed_at: string;
  status: string;
  expedited: boolean;
  total: number;
  grade: Grade;
  judged_weight: number;
  dimensions: OrderDimension[];
  batch_count: number;
  on_time_batches: number;
  accepted: number;
  rejected: number;
  iqc_results: string[];
  note: string;
  demo: boolean;
};

export type OrderScorecardsResult = {
  supplier: Supplier;
  tenant: { id: string; name: string; industry: string; plan: string; since: string } | null;
  benchmark: { price: number; basis: string };
  cards: OrderScorecard[];
  summary: {
    total: number;
    judged: number;
    average_score: number;
    worst_score: number;
    best_score: number;
    caveat: string;
  };
  assessment: QCDSTAssessment;
  demo: boolean;
};

export type EngageResult = {
  engagement: Engagement;
  created: boolean;
  supplier_name: string;
  message: string;
};

export type Tenant = { id: string; name: string; industry: string; plan: string; since: string };

export type SRMWorkspace = {
  tenant: Tenant | null;
  engagements: Engagement[];
  assessments: QCDSTAssessment[];
  stats: {
    suppliers: number;
    active: number;
    orders: number;
    open_orders: number;
    deliveries: number;
    on_time_rate: number;
    pass_rate: number;
    spend: number;
    quality_events: number;
  };
  demo: boolean;
  insights: { kind: string; level: string; title: string; detail: string }[];
};

export function fetchRanking(signal?: AbortSignal) {
  return get<{ items: PlatformRating[]; meta: Record<string, unknown> }>("/api/v1/qcdst/ranking", signal);
}

export function fetchQcdst(id: string, tenant?: string, signal?: AbortSignal) {
  const q = tenant ? `?tenant=${encodeURIComponent(tenant)}` : "";
  return get<QCDSTAssessment>(`/api/v1/qcdst/supplier/${encodeURIComponent(id)}${q}`, signal);
}

export function fetchWorkspace(tenant?: string, signal?: AbortSignal) {
  const q = tenant ? `?tenant=${encodeURIComponent(tenant)}` : "";
  return get<SRMWorkspace>(`/api/v1/srm/workspace${q}`, signal);
}

export function fetchTenants(signal?: AbortSignal) {
  return get<Tenant[]>("/api/v1/srm/tenants", signal);
}

export function seedDemo() {
  return post<{ suppliers: number; demo: boolean }>("/api/v1/srm/demo/seed", {});
}

export function clearDemo() {
  return post<{ cleared: boolean }>("/api/v1/srm/demo/clear", {});
}

// 寻源页一次拉走的评级索引，前端按 supplier_id 合并到搜索/图谱结果上。
export function fetchBriefs(tenant?: string, signal?: AbortSignal) {
  const q = tenant ? `?tenant=${encodeURIComponent(tenant)}` : "";
  return get<BriefIndex>(`/api/v1/qcdst/briefs${q}`, signal);
}

// 闭环体检：平台池有多大、能不能信、本企业贡献了多少。
export function fetchLoop(tenant?: string, signal?: AbortSignal) {
  const q = tenant ? `?tenant=${encodeURIComponent(tenant)}` : "";
  return get<LoopReport>(`/api/v1/qcdst/loop${q}`, signal);
}

// 把一家供应商引入本企业 SRM。source=sourcing 表示从公开寻源页引入。
export function engageSupplier(payload: {
  tenant_id?: string;
  tenant_name?: string;
  supplier_id: string;
  status?: string;
  category?: string;
  buyer_name?: string;
  source?: string;
  note?: string;
}) {
  return post<EngageResult>("/api/v1/srm/engagements", payload);
}

export function setEngagementStatus(tenantId: string, supplierId: string, status: string) {
  return post<Engagement>("/api/v1/srm/engagements/status", {
    tenant_id: tenantId,
    supplier_id: supplierId,
    status,
  });
}

// 逐单 QCDST 评分卡
export function fetchOrderScorecards(supplierId: string, tenant?: string, signal?: AbortSignal) {
  const q = `?supplier=${encodeURIComponent(supplierId)}${tenant ? `&tenant=${encodeURIComponent(tenant)}` : ""}`;
  return get<OrderScorecardsResult>(`/api/v1/srm/orders${q}`, signal);
}

export function ensureTenant(payload: { id?: string; name?: string; industry?: string }) {
  return post<{ id: string; name: string; industry: string; plan: string; since: string }>("/api/v1/srm/tenants", payload);
}

// ---------------- BOM ----------------

export type BomPart = {
  code: string;
  name: string;
  category: string;
  category_name: string;
  qty: string;
  critical: boolean;
  capability: string;
  note: string;
};

export type BomGroup = {
  code: string;
  name: string;
  role: string;
  critical: boolean;
  parts: BomPart[];
};

export type BomProductSummary = {
  code: string;
  name: string;
  sector: string;
  aliases: string[];
  summary: string;
  part_count: number;
  top_parts: number;
  groups: number;
  capabilities: string[];
};

export type BomFacet = {
  category: { code: string; name: string; description: string; key_capabilities: string[] };
  parts: number;
  products: string[];
  critical: number;
};

export type BomIndex = {
  items: BomProductSummary[];
  sectors: Record<string, string[]>;
  facets: BomFacet[];
  categories: { code: string; name: string; description: string }[];
};

export type BomDetail = {
  product: {
    code: string;
    name: string;
    sector: string;
    summary: string;
    aliases: string[];
    part_count: number;
    top_parts: number;
    capabilities: string[];
  };
  groups: BomGroup[];
  total_parts: number;
};

export function fetchBomIndex(signal?: AbortSignal) {
  return get<BomIndex>("/api/v1/bom/products", signal);
}

export function fetchBomDetail(code: string, signal?: AbortSignal) {
  return get<BomDetail>(`/api/v1/bom/products/${encodeURIComponent(code)}`, signal);
}

// ---------------- 部件 → 供应源 ----------------

// 覆盖风险：缺口 / 单点 / 偏窄 / 充足
export type CoverageRisk = "none" | "single" | "narrow" | "adequate";

export type PartCoverage = {
  category: { code: string; name: string; description: string; key_capabilities: string[] };
  parts: number;
  critical: boolean;
  suppliers: number;
  rated: number;
  engaged: number;
  best_score: number;
  best_fit: number;
  risk: CoverageRisk;
  risk_label: string;
  advice: string;
  top_supplier: string;
};

// 一家供应商对某小类的适配情况。
// 注意 fit_score 不含评级分：适配是"能不能做"，评级是"做得怎么样"，两者不能混。
export type PartSupplierMatch = {
  supplier_id: string;
  name: string;
  short_name: string;
  city: string;
  province: string;
  is_manufacturer: boolean;
  lead_time_days: number;
  moq: number;
  fit_score: number;
  fit_reasons: string[];
  hit_capabilities: string[];
  miss_capabilities: string[];
  capability_ratio: number;
  region_tag: string;
  main_products: string[];
  rating: PlatformRating | null;
  engagement: Engagement | null;
};

export type PartSupply = {
  coverage: PartCoverage;
  items: PartSupplierMatch[];
  demand_parts: BomPart[];
  capability: string;
  region: string;
  products: string[];
  note: string;
};

export type GroupSupply = {
  code: string;
  name: string;
  role: string;
  critical: boolean;
  coverages: PartCoverage[];
  risk: CoverageRisk;
};

export type SupplySummary = {
  categories: number;
  gaps: number;
  single_points: number;
  critical_gaps: number;
  coverage_rate: number;
  second_source_rate: number;
  headline: string;
};

export type ProductSupplyMatrix = {
  product: BomDetail["product"] & { groups?: BomGroup[] };
  groups: GroupSupply[];
  summary: SupplySummary;
  region: string;
  tenant_id: string;
  demo: boolean;
};

export type SupplierPartCoverage = {
  supplier: Supplier;
  categories: PartCoverage[];
  parts: {
    part: BomPart;
    product_code: string;
    product_name: string;
    group_name: string;
    direct: boolean;
    basis: string;
  }[];
  summary: {
    categories: number;
    parts: number;
    direct_parts: number;
    indirect_parts: number;
    critical: number;
    avg_rivalry: number;
    headline: string;
  };
};

export function fetchPartSupply(params: {
  category: string;
  tenant?: string;
  region?: string;
  capability?: string;
  product?: string;
}, signal?: AbortSignal) {
  const q = new URLSearchParams();
  q.set("category", params.category);
  if (params.tenant) q.set("tenant", params.tenant);
  if (params.region) q.set("region", params.region);
  if (params.capability) q.set("capability", params.capability);
  if (params.product) q.set("product", params.product);
  return get<PartSupply>(`/api/v1/bom/supply?${q.toString()}`, signal);
}

export function fetchProductSupplyMatrix(code: string, tenant?: string, region?: string, signal?: AbortSignal) {
  const q = new URLSearchParams();
  if (tenant) q.set("tenant", tenant);
  if (region) q.set("region", region);
  const suffix = q.toString() ? `?${q.toString()}` : "";
  return get<ProductSupplyMatrix>(`/api/v1/bom/products/${encodeURIComponent(code)}/supply${suffix}`, signal);
}

export function fetchSupplierParts(id: string, signal?: AbortSignal) {
  return get<SupplierPartCoverage>(`/api/v1/supplier/${encodeURIComponent(id)}/parts`, signal);
}

// ---------------- 数据开放策略 ----------------
// 规则要能被外部读到：接口返回的结构与后端 policy 包一一对应。
// 前端只做展示，不做裁剪判断——字段边界必须在服务端强制，前端隐藏等于没隐藏。

export type PolicyTier = {
  tier: string;
  label: string;
  who: string;
  how_to_get: string;
  can_see: string;
  data_right: string;
};

export type PolicyFieldGroup = {
  key: string;
  label: string;
  fields: string[];
  tier: string;
  reason: string;
};

export type DataPolicy = {
  version: string;
  positioning: string;
  principle: string[];
  tiers: PolicyTier[];
  fields: PolicyFieldGroup[];
  quotas: {
    max_page_size: number;
    deep_page_free_max: number;
    requests_per_min: number;
    detail_free_views: number;
  };
  export: { allowed: string[]; denied: string[]; why: string; alternative: string };
  prohibited: string[];
};

export function fetchDataPolicy(signal?: AbortSignal) {
  return get<DataPolicy>("/api/v1/meta/policy", signal);
}

// ---- 供应商五维档案(人·机·料·法·环):巡厂人工录入的事实,不是评分 ----

export type DeptHeadcount = { name: string; count: number };
export type MachineItem = { name: string; model: string; qty: number; age_year: number };

export type FourM1E = {
  supplier_id: string;
  // 人
  employee_total: number;
  rnd_count: number;
  departments: DeptHeadcount[];
  key_personnel: string;
  // 机
  factory_area: number;
  production_lines: number;
  machines: MachineItem[];
  automation_rate: number;
  capacity_utilization: number;
  // 料
  main_materials: string;
  dual_sourcing: string;
  iqc_method: string;
  material_compliance: string;
  // 法
  certifications: string[];
  design_capability: string;
  inspection_capability: string;
  process_docs: string;
  change_management: string;
  // 环
  env_certification: string;
  env_penalty: string;
  waste_treatment: string;
  location: string;
  // 留痕
  recorded_by: string;
  recorded_at: string;
};

export const EMPTY_FOUR_M1E = (supplierId: string): FourM1E => ({
  supplier_id: supplierId,
  employee_total: 0, rnd_count: 0, departments: [], key_personnel: "",
  factory_area: 0, production_lines: 0, machines: [], automation_rate: 0, capacity_utilization: 0,
  main_materials: "", dual_sourcing: "", iqc_method: "", material_compliance: "",
  certifications: [], design_capability: "", inspection_capability: "", process_docs: "", change_management: "",
  env_certification: "", env_penalty: "", waste_treatment: "", location: "",
  recorded_by: "", recorded_at: "",
});

export type FourM1EEnvelope = {
  data: FourM1E | null;
  history_count: number;
  last_recorded_by: string;
  last_recorded_at: string;
};

// 4m1e 的响应信封是 { data, history_count, ... },与 get() 默认剥一层的约定不同,
// 这里直接取完整 body,调用方才能同时拿到档案与留痕计数。
export async function fetchFourM1E(supplierId: string, signal?: AbortSignal): Promise<FourM1EEnvelope> {
  const response = await fetch(`${API}/api/v1/srm/4m1e/${encodeURIComponent(supplierId)}`, { signal });
  if (!response.ok) throw new Error(`请求失败（${response.status}）`);
  return (await response.json()) as FourM1EEnvelope;
}

export function fetchFourM1EHistory(supplierId: string, signal?: AbortSignal) {
  return get<FourM1E[]>(`/api/v1/srm/4m1e/${encodeURIComponent(supplierId)}/history`, signal);
}

// 保存走原始 fetch:错误信息在后端 body 的 error 字段里(校验失败原因),要原样带给录入人。
export async function saveFourM1E(rec: FourM1E): Promise<FourM1E> {
  const response = await fetch(`${API}/api/v1/srm/4m1e/${encodeURIComponent(rec.supplier_id)}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(rec),
  });
  const payload = await response.json();
  if (!response.ok) throw new Error(payload.error || `保存失败（${response.status}）`);
  return payload.data as FourM1E;
}

// ---- 美团式四维综合评分:QCDST 40% + 匹配 30% + 工商 15% + 4M1E 15% ----

export type OverallDimension = {
  key: "qcdst" | "match" | "registry" | "field" | string;
  label: string;
  score: number; // 0–100,无数据为 0
  weight: number;
  flag: "" | "insufficient_evidence" | "no_data" | string;
  note: string; // 一句话口径
};

export type OverallScore = {
  supplier_id?: string;
  supplier_name?: string;
  short_name?: string;
  credit_code?: string;
  city?: string;
  capabilities?: string[];
  total: number;
  dimensions: OverallDimension[];
  flags: string[];
  confidence: "high" | "medium" | "low" | string;
};

// 榜单条目:身份字段后端必返(附着在条目上),与搜索场景的可选 Overall 不同。
export type ScoringRankingItem = Omit<
  OverallScore,
  "supplier_id" | "supplier_name" | "short_name" | "credit_code" | "city" | "capabilities"
> & {
  supplier_id: string;
  supplier_name: string;
  short_name?: string;
  credit_code?: string;
  city?: string;
  capabilities?: string[];
};

export function fetchScoringRanking(capability?: string, signal?: AbortSignal) {
  const qs = capability ? `?capability=${encodeURIComponent(capability)}` : "";
  return get<{ items: ScoringRankingItem[]; total: number; page: number; page_size: number; capability: string; weights: Record<string, number> }>(
    `/api/v1/scoring/ranking${qs}`, signal,
  );
}

// 维度标记的展示文案:徽章 tooltip 与榜单标记共用。
export function flagLabel(flag: string): string {
  if (flag === "insufficient_evidence") return "证据不足";
  if (flag === "no_data") return "无数据";
  return "";
}

export type CapabilityMeta = { code: string; name: string; category: string; description: string };

export function fetchCapabilities(signal?: AbortSignal) {
  return get<CapabilityMeta[]>("/api/v1/catalog/capabilities", signal);
}
