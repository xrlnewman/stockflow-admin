package inventory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound         = errors.New("库存单据不存在")
	ErrAlreadyCompleted = errors.New("库存单据已经完成")
	ErrIdempotencyReuse = errors.New("幂等键已被其他请求使用")
)

type Warehouse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Address   string `json:"address"`
	StockKeep int    `json:"stockKeep"`
}

type Product struct {
	ID        string  `json:"id"`
	SKU       string  `json:"sku"`
	Name      string  `json:"name"`
	Category  string  `json:"category"`
	Warehouse string  `json:"warehouse"`
	Unit      string  `json:"unit"`
	Stock     int     `json:"stock"`
	MinStock  int     `json:"minStock"`
	Price     float64 `json:"price"`
	Status    string  `json:"status"`
}

type StockAlert struct {
	ProductID string `json:"productId"`
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	Warehouse string `json:"warehouse"`
	Stock     int    `json:"stock"`
	MinStock  int    `json:"minStock"`
	Severity  string `json:"severity"`
}

type PurchaseOrder struct {
	ID        string  `json:"id"`
	Supplier  string  `json:"supplier"`
	Warehouse string  `json:"warehouse"`
	Items     int     `json:"items"`
	Amount    float64 `json:"amount"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"createdAt"`
}

type SalesOrder struct {
	ID        string  `json:"id"`
	Customer  string  `json:"customer"`
	Warehouse string  `json:"warehouse"`
	Items     int     `json:"items"`
	Amount    float64 `json:"amount"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"createdAt"`
}

type Movement struct {
	ID        string `json:"id"`
	Product   string `json:"product"`
	Warehouse string `json:"warehouse"`
	Type      string `json:"type"`
	Quantity  int    `json:"quantity"`
	Source    string `json:"source"`
	CreatedAt string `json:"createdAt"`
}

type Dashboard struct {
	TodaySales      float64 `json:"todaySales"`
	PendingInbound  int     `json:"pendingInbound"`
	PendingOutbound int     `json:"pendingOutbound"`
	LowStock        int     `json:"lowStock"`
	StockValue      float64 `json:"stockValue"`
}

type Service struct {
	mu            sync.RWMutex
	warehouses    []Warehouse
	products      []Product
	purchases     []PurchaseOrder
	sales         []SalesOrder
	movements     []Movement
	idempotencies map[string]string
}

func NewService() *Service {
	now := time.Now().UTC().Format(time.RFC3339)
	return &Service{
		warehouses: []Warehouse{
			{ID: "wh-east", Name: "东城中心仓", Address: "东城工业园 18 号", StockKeep: 326},
			{ID: "wh-west", Name: "西郊备货仓", Address: "西郊物流园 6 号", StockKeep: 188},
		},
		products: []Product{
			{ID: "p-1001", SKU: "ST-1001", Name: "轻量通勤双肩包", Category: "箱包", Warehouse: "东城中心仓", Unit: "件", Stock: 126, MinStock: 40, Price: 169, Status: "normal"},
			{ID: "p-1002", SKU: "ST-1002", Name: "便携保温杯 480ml", Category: "家居", Warehouse: "东城中心仓", Unit: "件", Stock: 18, MinStock: 30, Price: 89, Status: "warning"},
			{ID: "p-1003", SKU: "ST-1003", Name: "有机棉基础 T 恤", Category: "服饰", Warehouse: "西郊备货仓", Unit: "件", Stock: 9, MinStock: 24, Price: 129, Status: "danger"},
			{ID: "p-1004", SKU: "ST-1004", Name: "桌面收纳盒套装", Category: "家居", Warehouse: "西郊备货仓", Unit: "套", Stock: 64, MinStock: 20, Price: 59, Status: "normal"},
		},
		purchases: []PurchaseOrder{
			{ID: "PO20260716001", Supplier: "清和供应链", Warehouse: "东城中心仓", Items: 3, Amount: 12800, Status: "待入库", CreatedAt: now},
			{ID: "PO20260715008", Supplier: "织物工坊", Warehouse: "西郊备货仓", Items: 2, Amount: 7360, Status: "已入库", CreatedAt: now},
		},
		sales: []SalesOrder{
			{ID: "SO20260716032", Customer: "星河生活馆", Warehouse: "东城中心仓", Items: 8, Amount: 3280, Status: "待发货", CreatedAt: now},
			{ID: "SO20260716031", Customer: "林先生", Warehouse: "西郊备货仓", Items: 2, Amount: 258, Status: "已完成", CreatedAt: now},
		},
		movements: []Movement{
			{ID: "MV20260716009", Product: "便携保温杯 480ml", Warehouse: "东城中心仓", Type: "出库", Quantity: 12, Source: "SO20260716029", CreatedAt: now},
			{ID: "MV20260716008", Product: "轻量通勤双肩包", Warehouse: "东城中心仓", Type: "入库", Quantity: 60, Source: "PO20260716001", CreatedAt: now},
		},
		idempotencies: make(map[string]string),
	}
}

func (s *Service) Dashboard(ctx context.Context) (Dashboard, error) {
	if err := ctx.Err(); err != nil {
		return Dashboard{}, fmt.Errorf("读取库存概览: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var low, pendingIn, pendingOut int
	var value float64
	for _, p := range s.products {
		value += float64(p.Stock) * p.Price
		if p.Stock <= p.MinStock {
			low++
		}
	}
	for _, order := range s.purchases {
		if order.Status == "待入库" {
			pendingIn++
		}
	}
	for _, order := range s.sales {
		if order.Status == "待发货" {
			pendingOut++
		}
	}
	return Dashboard{TodaySales: 3538, PendingInbound: pendingIn, PendingOutbound: pendingOut, LowStock: low, StockValue: value}, nil
}

func (s *Service) Warehouses(ctx context.Context) ([]Warehouse, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("读取仓库: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Warehouse(nil), s.warehouses...), nil
}

func (s *Service) Products(ctx context.Context, keyword string, page, pageSize int) ([]Product, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, fmt.Errorf("读取商品: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	filtered := make([]Product, 0, len(s.products))
	for _, p := range s.products {
		if keyword == "" || contains(p.Name, keyword) || contains(p.SKU, keyword) {
			filtered = append(filtered, p)
		}
	}
	return paginate(filtered, page, pageSize)
}

func (s *Service) Alerts(ctx context.Context, page, pageSize int) ([]StockAlert, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, fmt.Errorf("读取库存预警: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	alerts := make([]StockAlert, 0)
	for _, p := range s.products {
		if p.Stock <= p.MinStock {
			severity := "warning"
			if p.Stock*2 <= p.MinStock {
				severity = "danger"
			}
			alerts = append(alerts, StockAlert{ProductID: p.ID, SKU: p.SKU, Name: p.Name, Warehouse: p.Warehouse, Stock: p.Stock, MinStock: p.MinStock, Severity: severity})
		}
	}
	return paginate(alerts, page, pageSize)
}

func (s *Service) Purchases(ctx context.Context, page, pageSize int) ([]PurchaseOrder, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, fmt.Errorf("读取采购单: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return paginate(s.purchases, page, pageSize)
}

func (s *Service) Receive(ctx context.Context, id, idem string) (PurchaseOrder, error) {
	if err := ctx.Err(); err != nil {
		return PurchaseOrder{}, fmt.Errorf("确认入库: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if previous, ok := s.idempotencies[idem]; idem != "" && ok {
		if previous != id {
			return PurchaseOrder{}, ErrIdempotencyReuse
		}
	}
	for i := range s.purchases {
		if s.purchases[i].ID != id {
			continue
		}
		if s.purchases[i].Status == "已入库" {
			return s.purchases[i], ErrAlreadyCompleted
		}
		s.purchases[i].Status = "已入库"
		if idem != "" {
			s.idempotencies[idem] = id
		}
		s.movements = append([]Movement{{ID: "MV" + time.Now().UTC().Format("20060102150405"), Product: "采购入库", Warehouse: s.purchases[i].Warehouse, Type: "入库", Quantity: s.purchases[i].Items, Source: id, CreatedAt: time.Now().UTC().Format(time.RFC3339)}}, s.movements...)
		return s.purchases[i], nil
	}
	return PurchaseOrder{}, ErrNotFound
}

func (s *Service) Sales(ctx context.Context, page, pageSize int) ([]SalesOrder, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, fmt.Errorf("读取销售单: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return paginate(s.sales, page, pageSize)
}

func (s *Service) Ship(ctx context.Context, id, idem string) (SalesOrder, error) {
	if err := ctx.Err(); err != nil {
		return SalesOrder{}, fmt.Errorf("确认出库: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if previous, ok := s.idempotencies[idem]; idem != "" && ok {
		if previous != id {
			return SalesOrder{}, ErrIdempotencyReuse
		}
	}
	for i := range s.sales {
		if s.sales[i].ID != id {
			continue
		}
		if s.sales[i].Status == "已完成" {
			return s.sales[i], ErrAlreadyCompleted
		}
		s.sales[i].Status = "已完成"
		if idem != "" {
			s.idempotencies[idem] = id
		}
		s.movements = append([]Movement{{ID: "MV" + time.Now().UTC().Format("20060102150406"), Product: "销售出库", Warehouse: s.sales[i].Warehouse, Type: "出库", Quantity: s.sales[i].Items, Source: id, CreatedAt: time.Now().UTC().Format(time.RFC3339)}}, s.movements...)
		return s.sales[i], nil
	}
	return SalesOrder{}, ErrNotFound
}

func (s *Service) Movements(ctx context.Context, page, pageSize int) ([]Movement, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, fmt.Errorf("读取库存流水: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return paginate(s.movements, page, pageSize)
}

func contains(value, keyword string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(keyword))
}

func paginate[T any](items []T, page, pageSize int) ([]T, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return append([]T(nil), items[start:end]...), total, nil
}
