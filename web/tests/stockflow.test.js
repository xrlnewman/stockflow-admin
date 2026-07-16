import test from 'node:test';
import assert from 'node:assert/strict';
import { demoSnapshot, resolveAuthState } from '../src/api/client.js';

test('StockFlow 演示数据覆盖库存预警与订单闭环', () => {
  assert.equal(demoSnapshot.dashboard.lowStock, 2);
  assert.equal(demoSnapshot.purchases[0].status, '待入库');
  assert.equal(demoSnapshot.sales[0].status, '待发货');
  assert.equal(demoSnapshot.alerts[1].severity, 'danger');
});

test('认证状态在未配置 API 时仍允许离线演示', () => {
  assert.equal(resolveAuthState({}), 'offline-ready');
  assert.equal(resolveAuthState({ baseUrl: 'https://api.example.com' }), 'login');
  assert.equal(resolveAuthState({ baseUrl: 'https://api.example.com', token: 'demo' }), 'authenticated');
});
