package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/xrlnewman/stockflow-admin/server/internal/app/dispatch"
	"github.com/xrlnewman/stockflow-admin/server/internal/app/inventory"
	orderapp "github.com/xrlnewman/stockflow-admin/server/internal/app/order"
	"github.com/xrlnewman/stockflow-admin/server/internal/config"
	"github.com/xrlnewman/stockflow-admin/server/internal/domain"
	platformauth "github.com/xrlnewman/stockflow-admin/server/internal/platform/auth"
	"github.com/xrlnewman/stockflow-admin/server/internal/platform/cache"
	"github.com/xrlnewman/stockflow-admin/server/internal/platform/store"
)

type Dependencies struct {
	DB    *sql.DB
	Redis *cache.RedisLocker
}

type Server struct {
	cfg    config.Config
	store  *store.MemoryStore
	orders *orderapp.OrderService
	stock  *inventory.Service
	deps   Dependencies
}

func NewRouter(cfg config.Config, st *store.MemoryStore) *gin.Engine {
	return NewRouterWithDeps(cfg, st, Dependencies{})
}

func NewRouterWithDeps(cfg config.Config, st *store.MemoryStore, deps Dependencies) *gin.Engine {
	if st == nil {
		st = store.NewMemoryStore()
	}
	seed(st)
	var locker orderapp.Locker
	if deps.Redis != nil {
		locker = deps.Redis
	}
	s := &Server{cfg: cfg, store: st, orders: orderapp.NewService(st, locker), stock: inventory.NewService(), deps: deps}
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), traceMiddleware(), corsMiddleware(cfg.CORSOrigins))
	r.GET("/healthz", s.health)
	api := r.Group("/api/v1")
	api.POST("/auth/login", s.login)
	protected := api.Group("")
	protected.Use(s.requireAuth())
	protected.POST("/auth/refresh", s.refresh)
	protected.POST("/auth/logout", s.logout)
	protected.GET("/auth/me", s.me)
	protected.GET("/service-categories", s.categories)
	protected.GET("/stores", s.stores)
	protected.GET("/stores/:id/services", s.services)
	protected.GET("/stores/:id/slots", s.availability)
	protected.GET("/services", s.services)
	protected.GET("/services/:id", s.service)
	protected.GET("/availability", s.availability)
	protected.GET("/addresses", s.addresses)
	protected.POST("/addresses", s.addAddress)
	protected.POST("/orders", s.createOrder)
	protected.GET("/orders", s.ordersList)
	protected.POST("/appointments", s.createOrder)
	protected.GET("/appointments", s.ordersList)
	protected.PATCH("/appointments/:id/status", s.updateAppointmentStatus)
	protected.GET("/orders/:id", s.getOrder)
	protected.POST("/orders/:id/cancel", s.cancelOrder)
	protected.POST("/orders/:id/confirm", s.confirmOrder)
	protected.POST("/orders/:id/review", s.review)
	admin := protected.Group("/admin")
	admin.Use(requireRoles("admin", "dispatcher"))
	admin.GET("/orders", s.adminOrders)
	admin.POST("/orders/:id/assign", s.assignOrder)
	admin.GET("/dispatch/recommendations", s.recommendations)
	admin.GET("/audit-logs", s.auditLogs)
	admin.GET("/reviews", s.reviews)
	protected.GET("/dashboard/summary", s.dashboard)
	protected.GET("/dashboard", s.stockDashboard)
	protected.GET("/warehouses", s.stockWarehouses)
	protected.GET("/products", s.stockProducts)
	protected.GET("/stocks/alerts", s.stockAlerts)
	protected.GET("/purchase-orders", s.stockPurchases)
	protected.POST("/purchase-orders/:id/receive", s.stockReceive)
	protected.GET("/sales-orders", s.stockSales)
	protected.POST("/sales-orders/:id/ship", s.stockShip)
	protected.GET("/stock-movements", s.stockMovements)
	protected.GET("/technicians", s.technicians)
	workbench := protected.Group("/workbench")
	workbench.Use(requireRoles("technician", "admin"))
	workbench.POST("/orders/:id/accept", s.accept)
	workbench.POST("/orders/:id/arrive", s.arrive)
	workbench.POST("/orders/:id/start", s.start)
	workbench.POST("/orders/:id/proofs", s.proofs)
	workbench.POST("/orders/:id/complete", s.complete)
	return r
}

func corsMiddleware(rawOrigins string) gin.HandlerFunc {
	allowed := make(map[string]struct{})
	for _, origin := range strings.Split(rawOrigins, ",") {
		if value := strings.TrimSpace(origin); value != "" {
			allowed[value] = struct{}{}
		}
	}
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin == "" {
			c.Next()
			return
		}
		if _, ok := allowed[origin]; !ok {
			c.Next()
			return
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,Idempotency-Key,X-Trace-Id")
		c.Header("Access-Control-Expose-Headers", "X-Trace-Id")
		c.Header("Vary", "Origin")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func seed(st *store.MemoryStore) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("demo123456"), bcrypt.DefaultCost)
	st.SeedUser(store.User{ID: "user-demo", Phone: "13800000000", PasswordHash: string(hash), Name: "演示客户", Role: "customer"})
	st.SeedUser(store.User{ID: "admin-demo", Phone: "13900000000", PasswordHash: string(hash), Name: "运营管理员", Role: "admin"})
	st.SeedUser(store.User{ID: "tech-demo", Phone: "13700000000", PasswordHash: string(hash), Name: "演示员工", Role: "technician"})
	st.SeedTechnician(store.Technician{ID: "tech-demo", Name: "演示员工", Skills: []string{"cleaning"}, Areas: []string{"north"}, ShiftAvailable: true, Role: "technician"})
	date := time.Now().UTC().Add(24 * time.Hour)
	dateText := date.Format("2006-01-02")
	st.SeedService(store.Service{ID: "svc-clean", Name: "深度保洁", Skill: "cleaning", Area: "north", SlotCapacity: 10})
	st.SeedSlot(store.Slot{ID: "slot-demo-am", ServiceID: "svc-clean", Date: dateText, StartsAt: time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, time.UTC), Capacity: 10})
}

func traceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Trace-Id")
		if id == "" {
			id = "trace-" + uuid.NewString()
		}
		c.Set("traceId", id)
		c.Header("X-Trace-Id", id)
		c.Next()
	}
}
func (s *Server) envelope(c *gin.Context, code int, message string, data any) {
	trace, _ := c.Get("traceId")
	c.JSON(code, gin.H{"code": codeValue(code), "message": message, "data": data, "traceId": trace})
}
func codeValue(status int) any {
	if status < 400 {
		return 0
	}
	switch status {
	case http.StatusUnauthorized:
		return "AUTH_REQUIRED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusConflict:
		return "IDEMPOTENCY_CONFLICT"
	case http.StatusBadRequest:
		return "VALIDATION_FAILED"
	default:
		return "INTERNAL_ERROR"
	}
}

func (s *Server) health(c *gin.Context) {
	data := gin.H{"status": "ok", "mysql": "not_configured", "redis": "not_configured"}
	ctx, cancel := timeLimit(c)
	defer cancel()
	status := http.StatusOK
	if s.deps.DB != nil {
		if err := s.deps.DB.PingContext(ctx); err != nil {
			data["mysql"] = "unavailable"
			status = http.StatusServiceUnavailable
		} else {
			data["mysql"] = "ok"
		}
	}
	if s.deps.Redis != nil {
		if err := s.deps.Redis.Ping(ctx); err != nil {
			data["redis"] = "unavailable"
			status = http.StatusServiceUnavailable
		} else {
			data["redis"] = "ok"
		}
	}
	s.envelope(c, status, "ok", data)
}
func timeLimit(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), 2*time.Second)
}

func (s *Server) login(c *gin.Context) {
	var in struct {
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}
	if c.ShouldBindJSON(&in) != nil || in.Phone == "" || in.Password == "" {
		s.envelope(c, http.StatusBadRequest, "参数不完整", nil)
		return
	}
	u, err := s.store.UserByPhone(in.Phone)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)) != nil {
		s.envelope(c, http.StatusUnauthorized, "账号或密码错误", nil)
		return
	}
	token, err := platformauth.Issue(s.cfg.JWTSecret, u.ID, u.Role, 2*time.Hour)
	if err != nil {
		s.envelope(c, http.StatusInternalServerError, "登录失败", nil)
		return
	}
	s.store.AddAudit(store.AuditLog{ID: uuid.NewString(), ActorID: u.ID, Action: "login", Resource: "auth", Result: "success", CreatedAt: time.Now().UTC()})
	s.envelope(c, http.StatusOK, "ok", gin.H{"accessToken": token, "tokenType": "Bearer", "expiresIn": 7200, "user": gin.H{"id": u.ID, "name": u.Name, "role": u.Role}})
}

func (s *Server) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
		if raw == "" {
			s.envelope(c, http.StatusUnauthorized, "请先登录", nil)
			c.Abort()
			return
		}
		claims, err := platformauth.Parse(s.cfg.JWTSecret, raw)
		if err != nil {
			s.envelope(c, http.StatusUnauthorized, "登录已失效", nil)
			c.Abort()
			return
		}
		c.Set("claims", claims)
		c.Next()
	}
}
func requireRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, _ := c.Get("claims")
		role := claims.(platformauth.Claims).Role
		for _, allowed := range roles {
			if role == allowed {
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "没有权限", "data": nil, "traceId": c.GetString("traceId")})
		c.Abort()
	}
}
func claimsOf(c *gin.Context) platformauth.Claims {
	claims, _ := c.Get("claims")
	return claims.(platformauth.Claims)
}

func (s *Server) me(c *gin.Context) {
	u, err := s.store.UserByID(claimsOf(c).UserID)
	if err != nil {
		s.envelope(c, http.StatusUnauthorized, "用户不存在", nil)
		return
	}
	s.envelope(c, http.StatusOK, "ok", gin.H{"id": u.ID, "name": u.Name, "phone": u.Phone, "role": u.Role})
}
func (s *Server) refresh(c *gin.Context) {
	cl := claimsOf(c)
	token, _ := platformauth.Issue(s.cfg.JWTSecret, cl.UserID, cl.Role, 2*time.Hour)
	s.envelope(c, http.StatusOK, "ok", gin.H{"accessToken": token, "tokenType": "Bearer", "expiresIn": 7200})
}
func (s *Server) logout(c *gin.Context) { s.envelope(c, http.StatusOK, "ok", gin.H{}) }

func (s *Server) categories(c *gin.Context) {
	s.envelope(c, http.StatusOK, "ok", []gin.H{{"id": "home-service", "name": "门店预约"}})
}
func (s *Server) stores(c *gin.Context) {
	s.envelope(c, http.StatusOK, "ok", gin.H{"list": []gin.H{
		{"id": "store-riverside", "name": "滨江生活馆", "address": "江南大道 88 号", "distance": "0.8 km", "rating": 4.9},
		{"id": "store-lakeside", "name": "湖墅体验店", "address": "湖墅南路 20 号", "distance": "2.1 km", "rating": 4.8},
	}, "total": 2, "page": 1, "pageSize": 20})
}
func (s *Server) services(c *gin.Context) {
	values := s.store.Services()
	s.envelope(c, http.StatusOK, "ok", gin.H{"list": values, "total": len(values), "page": 1, "pageSize": len(values)})
}
func (s *Server) service(c *gin.Context) {
	value, err := s.store.ServiceByID(c.Param("id"))
	if err != nil {
		s.envelope(c, http.StatusNotFound, "服务不存在", nil)
		return
	}
	s.envelope(c, http.StatusOK, "ok", value)
}
func (s *Server) availability(c *gin.Context) {
	values := s.store.Slots(c.Query("serviceId"), c.Query("date"))
	data := make([]gin.H, 0, len(values))
	for _, slot := range values {
		data = append(data, gin.H{"id": slot.ID, "serviceId": slot.ServiceID, "date": slot.Date, "startsAt": slot.StartsAt, "capacity": slot.Capacity, "remaining": slot.Capacity - slot.Used})
	}
	s.envelope(c, http.StatusOK, "ok", data)
}
func (s *Server) addresses(c *gin.Context) {
	s.envelope(c, http.StatusOK, "ok", s.store.Addresses(claimsOf(c).UserID))
}
func (s *Server) addAddress(c *gin.Context) {
	var in store.Address
	if c.ShouldBindJSON(&in) != nil || in.ContactName == "" || in.Phone == "" || in.Detail == "" {
		s.envelope(c, http.StatusBadRequest, "参数不完整", nil)
		return
	}
	in.ID, in.UserID = uuid.NewString(), claimsOf(c).UserID
	s.store.AddAddress(in)
	s.store.AddAudit(store.AuditLog{ID: uuid.NewString(), ActorID: in.UserID, Action: "create_address", Resource: in.ID, Result: "success", CreatedAt: time.Now().UTC()})
	s.envelope(c, http.StatusCreated, "ok", in)
}
func (s *Server) technicians(c *gin.Context) {
	s.envelope(c, http.StatusOK, "ok", gin.H{"list": s.store.Technicians(), "total": len(s.store.Technicians()), "page": 1, "pageSize": 20})
}
func (s *Server) dashboard(c *gin.Context) {
	orders := s.store.Orders()
	completed := 0
	pending := 0
	for _, value := range orders {
		if value.State == domain.OrderCompleted {
			completed++
		}
		if value.State != domain.OrderCompleted && value.State != domain.OrderCancelled {
			pending++
		}
	}
	s.envelope(c, http.StatusOK, "ok", gin.H{"orders": len(orders), "completed": completed, "pending": pending, "completionRate": percentage(completed, len(orders)), "revenue": 0})
}
func percentage(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total)
}
func (s *Server) adminOrders(c *gin.Context) {
	values := s.store.Orders()
	status := domain.OrderState(c.Query("status"))
	if status != "" {
		filtered := values[:0]
		for _, value := range values {
			if value.State == status {
				filtered = append(filtered, value)
			}
		}
		values = filtered
	}
	s.envelope(c, http.StatusOK, "ok", gin.H{"list": values, "total": len(values), "page": 1, "pageSize": len(values)})
}

func (s *Server) createOrder(c *gin.Context) {
	var in orderapp.CreateInput
	if c.ShouldBindJSON(&in) != nil {
		s.envelope(c, http.StatusBadRequest, "参数不合法", nil)
		return
	}
	in.UserID = claimsOf(c).UserID
	in.IdempotencyKey = c.GetHeader("Idempotency-Key")
	order, err := s.orders.Create(c.Request.Context(), in)
	if err != nil {
		s.orderError(c, err)
		return
	}
	s.store.AddAudit(store.AuditLog{ID: uuid.NewString(), ActorID: in.UserID, Action: "create_order", Resource: order.ID, Result: "success", CreatedAt: time.Now().UTC()})
	s.envelope(c, http.StatusCreated, "ok", order)
}

func (s *Server) ordersList(c *gin.Context) {
	claims := claimsOf(c)
	values := s.store.Orders()
	if claims.Role == "customer" {
		owned := values[:0]
		for _, value := range values {
			if value.UserID == claims.UserID {
				owned = append(owned, value)
			}
		}
		values = owned
	}
	if status := domain.OrderState(c.Query("status")); status != "" {
		filtered := values[:0]
		for _, value := range values {
			if value.State == status {
				filtered = append(filtered, value)
			}
		}
		values = filtered
	}
	page, pageSize := 1, 20
	if raw := c.Query("page"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if raw := c.Query("pageSize"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}
	total := len(values)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	s.envelope(c, http.StatusOK, "ok", gin.H{"list": values[start:end], "total": total, "page": page, "pageSize": pageSize})
}

func (s *Server) getOrder(c *gin.Context) {
	order, err := s.store.OrderByID(c.Param("id"))
	if err != nil {
		s.envelope(c, http.StatusNotFound, "订单不存在", nil)
		return
	}
	cl := claimsOf(c)
	if cl.Role == "customer" && order.UserID != cl.UserID {
		s.envelope(c, http.StatusForbidden, "没有权限", nil)
		return
	}
	s.envelope(c, http.StatusOK, "ok", order)
}
func (s *Server) cancelOrder(c *gin.Context) {
	if !s.requireCustomerOrder(c) {
		return
	}
	s.transition(c, domain.OrderCancelled)
}
func (s *Server) confirmOrder(c *gin.Context) {
	if !s.requireCustomerOrder(c) {
		return
	}
	s.transition(c, domain.OrderCompleted)
}
func (s *Server) updateAppointmentStatus(c *gin.Context) {
	var in struct {
		Status string `json:"status"`
	}
	if c.ShouldBindJSON(&in) != nil || in.Status == "" {
		s.envelope(c, http.StatusBadRequest, "预约状态不能为空", nil)
		return
	}
	state := domain.OrderState(in.Status)
	switch state {
	case domain.OrderPendingConfirmation, domain.OrderPendingDispatch, domain.OrderAssigned, domain.OrderEnRoute, domain.OrderServing, domain.OrderPendingCustomerConfirmation, domain.OrderCompleted, domain.OrderCancelled:
		s.transition(c, state)
	default:
		s.envelope(c, http.StatusBadRequest, "预约状态不合法", nil)
	}
}
func (s *Server) review(c *gin.Context) {
	if !s.requireCustomerOrder(c) {
		return
	}
	order, err := s.store.OrderByID(c.Param("id"))
	if err != nil {
		s.envelope(c, http.StatusNotFound, "订单不存在", nil)
		return
	}
	if order.State != domain.OrderCompleted {
		s.envelope(c, http.StatusBadRequest, "订单完成后才可以评价", nil)
		return
	}
	var in struct {
		Rating  int    `json:"rating"`
		Content string `json:"content"`
	}
	if c.ShouldBindJSON(&in) != nil || in.Rating < 1 || in.Rating > 5 || strings.TrimSpace(in.Content) == "" {
		s.envelope(c, http.StatusBadRequest, "评价参数不合法", nil)
		return
	}
	review := store.Review{ID: uuid.NewString(), OrderID: order.ID, UserID: claimsOf(c).UserID, Rating: in.Rating, Content: in.Content, CreatedAt: time.Now().UTC()}
	s.store.AddReview(review)
	s.store.AddAudit(store.AuditLog{ID: uuid.NewString(), ActorID: review.UserID, Action: "create_review", Resource: review.ID, Result: "success", CreatedAt: review.CreatedAt})
	s.envelope(c, http.StatusCreated, "ok", review)
}
func (s *Server) transition(c *gin.Context, to domain.OrderState) {
	order, err := s.orders.Transition(c.Request.Context(), c.Param("id"), claimsOf(c).UserID, to)
	if err != nil {
		s.orderError(c, err)
		return
	}
	s.store.AddAudit(store.AuditLog{ID: uuid.NewString(), ActorID: claimsOf(c).UserID, Action: "transition_order", Resource: order.ID, Result: string(to), CreatedAt: time.Now().UTC()})
	s.envelope(c, http.StatusOK, "ok", order)
}
func (s *Server) assignOrder(c *gin.Context) {
	var in struct {
		TechnicianID string `json:"technicianId"`
	}
	if c.ShouldBindJSON(&in) != nil || in.TechnicianID == "" {
		s.envelope(c, http.StatusBadRequest, "参数不合法", nil)
		return
	}
	order, err := s.orders.Assign(c.Request.Context(), c.Param("id"), claimsOf(c).UserID, in.TechnicianID)
	if err != nil {
		s.orderError(c, err)
		return
	}
	s.store.AddAudit(store.AuditLog{ID: uuid.NewString(), ActorID: claimsOf(c).UserID, Action: "assign", Resource: order.ID, Result: "success", CreatedAt: time.Now().UTC()})
	s.envelope(c, http.StatusOK, "ok", order)
}
func (s *Server) recommendations(c *gin.Context) {
	candidates := make([]dispatch.TechnicianCandidate, 0)
	for _, t := range s.store.Technicians() {
		candidates = append(candidates, dispatch.TechnicianCandidate{ID: t.ID, Skills: t.Skills, Areas: t.Areas, ShiftAvailable: t.ShiftAvailable, Load: t.Load})
	}
	s.envelope(c, http.StatusOK, "ok", dispatch.RankTechnicians(candidates, "cleaning", "north"))
}
func (s *Server) auditLogs(c *gin.Context) {
	s.envelope(c, http.StatusOK, "ok", gin.H{"list": s.store.Audits(), "total": len(s.store.Audits()), "page": 1, "pageSize": 20})
}
func (s *Server) reviews(c *gin.Context) {
	values := s.store.Reviews()
	s.envelope(c, http.StatusOK, "ok", gin.H{"list": values, "total": len(values), "page": 1, "pageSize": len(values)})
}
func (s *Server) accept(c *gin.Context) {
	if !s.requireTechnicianOrder(c) {
		return
	}
	s.transition(c, domain.OrderEnRoute)
}
func (s *Server) arrive(c *gin.Context) {
	if !s.requireTechnicianOrder(c) {
		return
	}
	s.transition(c, domain.OrderServing)
}
func (s *Server) start(c *gin.Context) {
	if !s.requireTechnicianOrder(c) {
		return
	}
	s.transition(c, domain.OrderPendingCustomerConfirmation)
}
func (s *Server) proofs(c *gin.Context) {
	if !s.requireTechnicianOrder(c) {
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		s.envelope(c, http.StatusBadRequest, "请使用 multipart/form-data 上传凭证", nil)
		return
	}
	note := c.PostForm("note")
	result := make([]store.Proof, 0)
	for _, kind := range []string{"before", "after"} {
		for _, header := range form.File[kind] {
			proof := store.Proof{ID: uuid.NewString(), OrderID: c.Param("id"), Kind: kind, Filename: header.Filename, Note: note, CreatedAt: time.Now().UTC()}
			s.store.AddProof(proof)
			result = append(result, proof)
		}
	}
	if len(result) == 0 {
		s.envelope(c, http.StatusBadRequest, "至少上传一张服务凭证", nil)
		return
	}
	s.store.AddAudit(store.AuditLog{ID: uuid.NewString(), ActorID: claimsOf(c).UserID, Action: "upload_proof", Resource: c.Param("id"), Result: "success", CreatedAt: time.Now().UTC()})
	s.envelope(c, http.StatusCreated, "ok", result)
}
func (s *Server) complete(c *gin.Context) {
	if !s.requireTechnicianOrder(c) {
		return
	}
	s.transition(c, domain.OrderCompleted)
}
func (s *Server) requireTechnicianOrder(c *gin.Context) bool {
	claims := claimsOf(c)
	if claims.Role != "technician" {
		return true
	}
	order, err := s.store.OrderByID(c.Param("id"))
	if err != nil {
		s.envelope(c, http.StatusNotFound, "订单不存在", nil)
		return false
	}
	if order.TechnicianID != claims.UserID {
		s.envelope(c, http.StatusForbidden, "只能操作分配给自己的订单", nil)
		return false
	}
	return true
}

func (s *Server) requireCustomerOrder(c *gin.Context) bool {
	claims := claimsOf(c)
	if claims.Role != "customer" {
		return true
	}
	order, err := s.store.OrderByID(c.Param("id"))
	if err != nil {
		s.envelope(c, http.StatusNotFound, "订单不存在", nil)
		return false
	}
	if order.UserID != claims.UserID {
		s.envelope(c, http.StatusForbidden, "只能操作自己的订单", nil)
		return false
	}
	return true
}
func (s *Server) orderError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, orderapp.ErrSlotUnavailable):
		s.envelope(c, http.StatusConflict, "预约时段已满", nil)
	case errors.Is(err, store.ErrIdempotencyConflict):
		s.envelope(c, http.StatusConflict, "幂等键已被其他请求使用", nil)
	case errors.Is(err, domain.ErrOrderStateInvalid):
		s.envelope(c, http.StatusBadRequest, "订单状态不允许该操作", nil)
	default:
		s.envelope(c, http.StatusBadRequest, err.Error(), nil)
	}
}
