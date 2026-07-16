const DEFAULT_API_BASE_URL = '';

export const demoSnapshot = Object.freeze({
  dashboard: Object.freeze({ todaySales: 3538, pendingInbound: 1, pendingOutbound: 1, lowStock: 2, stockValue: 46820 }),
  warehouses: Object.freeze([{ id: 'wh-east', name: '东城中心仓', address: '东城工业园 18 号', stockKeep: 326 }, { id: 'wh-west', name: '西郊备货仓', address: '西郊物流园 6 号', stockKeep: 188 }]),
  alerts: Object.freeze([{ productId: 'p-1002', sku: 'ST-1002', name: '便携保温杯 480ml', warehouse: '东城中心仓', stock: 18, minStock: 30, severity: 'warning' }, { productId: 'p-1003', sku: 'ST-1003', name: '有机棉基础 T 恤', warehouse: '西郊备货仓', stock: 9, minStock: 24, severity: 'danger' }]),
  products: Object.freeze([{ id: 'p-1001', sku: 'ST-1001', name: '轻量通勤双肩包', category: '箱包', warehouse: '东城中心仓', stock: 126, minStock: 40, price: 169, status: 'normal' }, { id: 'p-1002', sku: 'ST-1002', name: '便携保温杯 480ml', category: '家居', warehouse: '东城中心仓', stock: 18, minStock: 30, price: 89, status: 'warning' }, { id: 'p-1003', sku: 'ST-1003', name: '有机棉基础 T 恤', category: '服饰', warehouse: '西郊备货仓', stock: 9, minStock: 24, price: 129, status: 'danger' }, { id: 'p-1004', sku: 'ST-1004', name: '桌面收纳盒套装', category: '家居', warehouse: '西郊备货仓', stock: 64, minStock: 20, price: 59, status: 'normal' }]),
  purchases: Object.freeze([{ id: 'PO20260716001', supplier: '清和供应链', warehouse: '东城中心仓', items: 3, amount: 12800, status: '待入库' }, { id: 'PO20260715008', supplier: '织物工坊', warehouse: '西郊备货仓', items: 2, amount: 7360, status: '已入库' }]),
  sales: Object.freeze([{ id: 'SO20260716032', customer: '星河生活馆', warehouse: '东城中心仓', items: 8, amount: 3280, status: '待发货' }, { id: 'SO20260716031', customer: '林先生', warehouse: '西郊备货仓', items: 2, amount: 258, status: '已完成' }]),
  movements: Object.freeze([{ id: 'MV20260716009', product: '便携保温杯 480ml', warehouse: '东城中心仓', type: '出库', quantity: 12, source: 'SO20260716029' }, { id: 'MV20260716008', product: '轻量通勤双肩包', warehouse: '东城中心仓', type: '入库', quantity: 60, source: 'PO20260716001' }]),
});

function envApiBaseUrl() { return typeof import.meta !== 'undefined' && import.meta.env ? import.meta.env.VITE_API_BASE_URL || DEFAULT_API_BASE_URL : DEFAULT_API_BASE_URL; }
export function resolveAuthState({ baseUrl = '', token = '' } = {}) { return String(token).trim() ? 'authenticated' : String(baseUrl).trim() ? 'login' : 'offline-ready'; }
function joinUrl(baseUrl, path) { return `${baseUrl.replace(/\/$/, '')}${path}`; }
function readEnvelope(body, response) { if (!response.ok || !body || body.code !== 0) throw new Error(body?.message || `API 请求失败（${response.status}）`); return body.data; }

export function createApiClient({ baseUrl = envApiBaseUrl(), token: initialToken = '', storage = typeof globalThis.localStorage !== 'undefined' ? globalThis.localStorage : null, fetchImpl = globalThis.fetch } = {}) {
  let token = initialToken || storage?.getItem?.('stockflow_access_token') || '';
  async function request(path, options = {}) {
    if (!baseUrl || typeof fetchImpl !== 'function') throw new Error('API 未配置');
    const headers = { Accept: 'application/json', ...(options.body ? { 'Content-Type': 'application/json' } : {}), ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(options.headers || {}) };
    const response = await fetchImpl(joinUrl(baseUrl, `/api/v1${path}`), { ...options, headers });
    let body; try { body = await response.json(); } catch { throw new Error('API 返回格式错误'); }
    return readEnvelope(body, response);
  }
  async function fallback(name, value, action) { try { return { source: 'api', data: await action() }; } catch (error) { console.warn(`[StockFlow] ${name} 回退演示数据`, error); return { source: 'demo', data: value }; } }
  async function login(credentials) { const data = await request('/auth/login', { method: 'POST', body: JSON.stringify(credentials) }); if (!data?.accessToken) throw new Error('登录响应缺少访问令牌'); token = data.accessToken; storage?.setItem?.('stockflow_access_token', token); return data; }
  async function logout() { if (baseUrl && token) { try { await request('/auth/logout', { method: 'POST' }); } catch (error) { console.warn('[StockFlow] 退出接口失败', error); } } token = ''; storage?.removeItem?.('stockflow_access_token'); }
  async function dashboard() { return fallback('dashboard', demoSnapshot.dashboard, () => request('/dashboard')); }
  async function warehouses() { return fallback('warehouses', { list: demoSnapshot.warehouses, total: 2 }, () => request('/warehouses')); }
  async function products() { return fallback('products', { list: demoSnapshot.products, total: demoSnapshot.products.length }, () => request('/products?page=1&pageSize=50')); }
  async function alerts() { return fallback('alerts', { list: demoSnapshot.alerts, total: 2 }, () => request('/stocks/alerts')); }
  async function purchases() { return fallback('purchases', { list: demoSnapshot.purchases, total: 2 }, () => request('/purchase-orders')); }
  async function sales() { return fallback('sales', { list: demoSnapshot.sales, total: 2 }, () => request('/sales-orders')); }
  async function movements() { return fallback('movements', { list: demoSnapshot.movements, total: 2 }, () => request('/stock-movements')); }
  async function mutate(path, id, name) { return fallback(name, (demoSnapshot[name] || []).find((item) => item.id === id) || {}, () => request(`${path}/${id}/${name === 'purchases' ? 'receive' : 'ship'}`, { method: 'POST', headers: { 'Idempotency-Key': `stockflow-${name}-${id}` }, body: '{}' })); }
  return { login, logout, dashboard, warehouses, products, alerts, purchases, sales, movements, receivePurchase: (id) => mutate('/purchase-orders', id, 'purchases'), shipSale: (id) => mutate('/sales-orders', id, 'sales'), getToken: () => token, isConfigured: () => Boolean(baseUrl) };
}
