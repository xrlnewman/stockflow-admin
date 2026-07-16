# StockFlow API Contract v1

## 统一响应

```json
{ "code": 0, "message": "ok", "data": {} }
```

列表接口的 `data` 统一为 `{ "list": [], "total": 0, "page": 1, "pageSize": 20 }`。所有写接口接受 `Idempotency-Key`，同一业务单据重复提交不会重复生成库存流水。

## 运行时依赖

- MySQL 8.4：保存商品、仓库、单据和流水；
- Redis 8：幂等键与短期缓存；
- 未配置依赖时，开发环境使用内存演示数据，并在健康接口中返回未配置状态。

演示数据不代表真实客户或交易，生产部署必须配置 `MYSQL_DSN`、`REDIS_ADDR`、`JWT_SECRET`。
