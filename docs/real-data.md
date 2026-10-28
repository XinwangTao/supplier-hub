# 真实供应商数据的接入方式

这个系统**不再内置任何生成的供应商数据**。默认启动后是一张空网，直到你接入真实数据源。

判断标准很简单：`GET /api/v1/meta/dataset` 返回 `is_real: true`，界面上不会出现红色警示条。

---

## 方式一：导入工商信息表（最快，不用写代码）

1. 从企查查 / 天眼查 / 启信宝 / 国家企业信用信息公示系统，把目标企业导出成 Excel，另存为 CSV。
2. 表头不必改，常见写法都能自动对齐（见下方映射表）。
3. 放到 `backend/data/suppliers.csv`，启动：

```bash
cd backend
SUPPLIER_SOURCE=csv SUPPLIER_CSV=data/suppliers.csv go run ./cmd/server
```

空模板在 `backend/data/suppliers.template.csv`，列名照抄即可。

### 列名自动映射

| 内部字段 | 可接受的表头写法 |
| --- | --- |
| 企业名称 | 企业名称 / 公司名称 / 供应商名称 / company_name |
| 统一社会信用代码 | 统一社会信用代码 / 社会信用代码 / 纳税人识别号 / uscc |
| 法定代表人 | 法定代表人 / 法人代表 / 法人 / legal_person |
| 经营状态 | 经营状态 / 登记状态 / 企业状态 / status |
| 企业类型 | 企业类型 / 公司类型 / company_type |
| 注册地址 | 注册地址 / 企业地址 / 住所 / address |
| 成立日期 | 成立日期 / 成立时间 / 注册时间 / founded_at |
| 注册资本 | 注册资本 / 注册资金 / capital（支持「1200万人民币」「1.5亿元」） |
| 参保人数 | 参保人数 / 社保人数 |
| 员工人数 | 员工人数 / 人员规模 / 员工总数 |
| 经营范围 | 经营范围 / business_scope / scope |
| 省 / 市 / 区县 | 省份 / 城市 / 区县（留空则从注册地址自动拆） |

其他列（纳税等级、进出口权、股东、主要人员等）如果表里有，会被一并读入；没有就留空，系统不会补造。

---

## 方式二：企查查开放平台 API（可批量、可定时更新）

1. 在 [企查查开放平台](https://openapi.qcc.com/) 申请账号，拿到 `key` 与 `secretKey`。
2. 把要查的企业全称写成一行一个，存成 `backend/data/companies.txt`（格式见 `companies.example.txt`）。
3. 启动：

```bash
cd backend
SUPPLIER_SOURCE=qcc \
QCC_KEY=你的key \
QCC_SECRET_KEY=你的secret \
SUPPLIER_NAMES=data/companies.txt \
go run ./cmd/server
```

要点：

- 结果会缓存到 `data/cache/qcc.json`，默认 24 小时有效，避免重复计费。改 `SUPPLIER_CACHE_TTL_HOURS` 调整，设 `0` 表示永久使用缓存。
- 接口按营业执照全称精确查询，名称必须与工商登记一致。
- 每条记录都带 `data_source = 企查查开放平台`、`last_verified_at`、`source_url`（可点回企查查原始页），档案面板里会显示。
- 单条失败不会污染整批：失败原因会写进 `/api/v1/meta/dataset` 的 `skipped` 字段。

> 天眼查、启信慧眼、水滴征信的接入同理：实现一个返回 `ingest.Record` 的客户端，在 `repository/source.go` 里加一个分支即可。接口形状已经定义好，不需要改上层。

---

## 工艺能力是怎么归到真实企业上的

真实企业没有"能力标签"，所以系统用**经营范围 + 企业名称**做关键词归类（`internal/ingest/classify.go`）：

- 每类工艺能力有一组判定词，例如 `pcba` ← SMT / 贴片 / 电路板组装 / 波峰焊。
- 命中词越多排名越前，最多取 3 个能力。
- 「整机组装 / 检测 / 包装」这类泛化词只有在没有更具体命中时才启用，避免所有企业都被归到组装。
- **一个都没命中就返回空**——宁可在图上缺一个点，也不挂错链。

想提高归类准确率，最直接的办法是把判定词改成你自己行业的说法：

```go
// internal/ingest/classify.go
"pcba": {"SMT", "贴片", "PCBA", "电子组装", "插件", "波峰焊"},
```

---

## 数据真实性约定

- 缺字段就留空，界面上显示"—"，绝不填看起来合理的值。
- 经营状态异常（注销/吊销/经营异常）会直接映射成风险等级与风险标签，不做美化。
- 没有统一社会信用代码的记录，用名称哈希生成稳定 ID，并在 `skipped` 里记一笔，提示补齐信用代码。
- `SUPPLIER_SOURCE=mock` 只用于开发联调，界面会全程挂红色警示条。
