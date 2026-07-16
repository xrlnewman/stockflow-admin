# StockFlow 运营后台

Vue 3 + Vite 的 StockFlow 进销存运营工作台，连接同仓库中的 Go Gin API。页面覆盖经营总览、商品中心、采购入库、销售出库、库存预警和库存流水。

```bash
npm install
npm test
npm run dev
```

未配置 `VITE_API_BASE_URL` 时自动进入离线演示，演示数据为虚构内容。配置后端地址并登录即可请求 `/api/v1`。
