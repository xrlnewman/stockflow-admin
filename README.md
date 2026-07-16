# StockFlow Admin

StockFlow 是一个免费开源的进销存运营后台与 Go Gin API，覆盖商品、仓库、采购入库、销售出库、低库存预警和库存流水。后台和服务端放在同一个仓库，方便小团队直接部署。

## 关联仓库

- [StockFlow Miniapp](https://github.com/xrlnewman/stockflow-miniapp)
- [许汝林个人博客项目页](https://field-notes-2fi.pages.dev/projects/stockflow-platform/)

## 技术栈

- Go 1.25 + Gin
- MySQL 8.4
- Redis 8
- Vue 3 + Vite

## 本地运行

API：

```bash
cd server
go vet ./...
go test ./...
go run ./cmd/api
```

后台：

```bash
cd web
npm install
npm test
npm run dev
```

完整基础设施示例在 `deploy/docker-compose.yml`，默认只使用本地演示数据；配置 `MYSQL_DSN`、`REDIS_ADDR` 和 `JWT_SECRET` 后可接入真实环境。密钥不能写入仓库。

## API 摘要

所有接口返回 `{ code, message, data }`，写接口支持 `Idempotency-Key`：

| Method | URL | 用途 |
| --- | --- | --- |
| GET | `/api/v1/dashboard` | 今日销售、待入库、待发货和库存总值 |
| GET | `/api/v1/warehouses` | 仓库列表 |
| GET | `/api/v1/products` | 商品分页与关键词搜索 |
| GET | `/api/v1/stocks/alerts` | 低库存预警 |
| GET/POST | `/api/v1/purchase-orders`、`/api/v1/purchase-orders/:id/receive` | 采购与入库 |
| GET/POST | `/api/v1/sales-orders`、`/api/v1/sales-orders/:id/ship` | 销售与出库 |
| GET | `/api/v1/stock-movements` | 库存流水 |

默认演示管理员：`13900000000` / `demo123456`。仅用于本地预览，生产环境必须替换。

## 许可

MIT © xrlnewman
