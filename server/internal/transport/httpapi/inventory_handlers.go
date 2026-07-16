package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/xrlnewman/stockflow-admin/server/internal/app/inventory"
)

func pageParams(c *gin.Context) (int, int) {
	page, pageSize := 1, 20
	if value, err := strconv.Atoi(c.Query("page")); err == nil && value > 0 {
		page = value
	}
	if value, err := strconv.Atoi(c.Query("pageSize")); err == nil && value > 0 && value <= 100 {
		pageSize = value
	}
	return page, pageSize
}

func (s *Server) stockDashboard(c *gin.Context) {
	data, err := s.stock.Dashboard(c.Request.Context())
	if err != nil {
		s.envelope(c, http.StatusInternalServerError, "读取库存概览失败", nil)
		return
	}
	s.envelope(c, http.StatusOK, "ok", data)
}

func (s *Server) stockWarehouses(c *gin.Context) {
	data, err := s.stock.Warehouses(c.Request.Context())
	if err != nil {
		s.envelope(c, http.StatusInternalServerError, "读取仓库失败", nil)
		return
	}
	s.envelope(c, http.StatusOK, "ok", gin.H{"list": data, "total": len(data), "page": 1, "pageSize": len(data)})
}

func (s *Server) stockProducts(c *gin.Context) {
	page, pageSize := pageParams(c)
	data, total, err := s.stock.Products(c.Request.Context(), c.Query("keyword"), page, pageSize)
	if err != nil {
		s.envelope(c, http.StatusInternalServerError, "读取商品失败", nil)
		return
	}
	s.envelope(c, http.StatusOK, "ok", gin.H{"list": data, "total": total, "page": page, "pageSize": pageSize})
}

func (s *Server) stockAlerts(c *gin.Context) {
	page, pageSize := pageParams(c)
	data, total, err := s.stock.Alerts(c.Request.Context(), page, pageSize)
	if err != nil {
		s.envelope(c, http.StatusInternalServerError, "读取库存预警失败", nil)
		return
	}
	s.envelope(c, http.StatusOK, "ok", gin.H{"list": data, "total": total, "page": page, "pageSize": pageSize})
}

func (s *Server) stockPurchases(c *gin.Context) {
	page, pageSize := pageParams(c)
	data, total, err := s.stock.Purchases(c.Request.Context(), page, pageSize)
	if err != nil {
		s.envelope(c, http.StatusInternalServerError, "读取采购单失败", nil)
		return
	}
	s.envelope(c, http.StatusOK, "ok", gin.H{"list": data, "total": total, "page": page, "pageSize": pageSize})
}

func (s *Server) stockReceive(c *gin.Context) {
	data, err := s.stock.Receive(c.Request.Context(), c.Param("id"), c.GetHeader("Idempotency-Key"))
	if err != nil {
		s.stockMutationError(c, err, "确认入库失败")
		return
	}
	s.envelope(c, http.StatusOK, "入库已确认", data)
}

func (s *Server) stockSales(c *gin.Context) {
	page, pageSize := pageParams(c)
	data, total, err := s.stock.Sales(c.Request.Context(), page, pageSize)
	if err != nil {
		s.envelope(c, http.StatusInternalServerError, "读取销售单失败", nil)
		return
	}
	s.envelope(c, http.StatusOK, "ok", gin.H{"list": data, "total": total, "page": page, "pageSize": pageSize})
}

func (s *Server) stockShip(c *gin.Context) {
	data, err := s.stock.Ship(c.Request.Context(), c.Param("id"), c.GetHeader("Idempotency-Key"))
	if err != nil {
		s.stockMutationError(c, err, "确认出库失败")
		return
	}
	s.envelope(c, http.StatusOK, "出库已确认", data)
}

func (s *Server) stockMovements(c *gin.Context) {
	page, pageSize := pageParams(c)
	data, total, err := s.stock.Movements(c.Request.Context(), page, pageSize)
	if err != nil {
		s.envelope(c, http.StatusInternalServerError, "读取库存流水失败", nil)
		return
	}
	s.envelope(c, http.StatusOK, "ok", gin.H{"list": data, "total": total, "page": page, "pageSize": pageSize})
}

func (s *Server) stockMutationError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, inventory.ErrNotFound):
		s.envelope(c, http.StatusNotFound, "库存单据不存在", nil)
	case errors.Is(err, inventory.ErrAlreadyCompleted):
		s.envelope(c, http.StatusConflict, "库存单据已经完成", nil)
	case errors.Is(err, inventory.ErrIdempotencyReuse):
		s.envelope(c, http.StatusConflict, "幂等键已被其他单据使用", nil)
	default:
		s.envelope(c, http.StatusBadRequest, fallback, nil)
	}
}
