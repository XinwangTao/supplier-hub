# 供应商寻源台

AI 驱动的供应商寻源 MVP，覆盖自然语言寻源、多维筛选、可解释排序、供应商画像和实地审核清单生成。

## 快速启动

```bash
cd backend
go run ./cmd/server
```

后端默认监听 `http://localhost:8080`，默认使用内存 Mock 数据，不依赖外部 API 或数据库。

完整环境：

```bash
docker compose up --build
```

前端地址：`http://localhost:3000`。后端地址：`http://localhost:8080`。

## 核心接口

- `GET /healthz`
- `POST /api/v1/suppliers/search`
- `GET /api/v1/suppliers/:id`
- `POST /api/v1/suppliers/:id/audit-checklist`

## 架构说明

`SupplierRepository` 是数据访问边界，当前实现为内存 Mock，后续可替换为 PostgreSQL/sqlc 或第三方企业数据适配器。`LLMClient` 是 AI 边界，未配置兼容 OpenAI 接口时使用确定性本地解析和模板清单，保证演示链路稳定。

## 环境变量

后端支持：`HTTP_ADDR`、`CORS_ORIGINS`、`AI_BASE_URL`、`AI_API_KEY`、`AI_MODEL`。数据库迁移位于 `backend/migrations`，sqlc 查询草稿位于 `backend/queries`。
