package store

import (
	"errors"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/xrlnewman/stockflow-admin/server/internal/domain"
)

var (
	ErrNotFound            = errors.New("resource not found")
	ErrIdempotencyConflict = errors.New("idempotency key belongs to another request")
)

type User struct {
	ID           string
	Phone        string
	PasswordHash string
	Name         string
	Role         string
}

type Service struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CategoryID   string `json:"categoryId"`
	Skill        string `json:"skill"`
	Area         string `json:"area"`
	SlotCapacity int    `json:"slotCapacity"`
}

type Slot struct {
	ID        string
	ServiceID string
	Date      string
	StartsAt  time.Time
	Capacity  int
	Used      int
}

type Address struct {
	ID          string `json:"id"`
	UserID      string `json:"userId"`
	ContactName string `json:"contactName"`
	Phone       string `json:"phone"`
	Detail      string `json:"detail"`
}

type Review struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"orderId"`
	UserID    string    `json:"userId"`
	Rating    int       `json:"rating"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type Proof struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"orderId"`
	Kind      string    `json:"kind"`
	Filename  string    `json:"filename"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type Technician struct {
	ID             string
	Name           string
	Skills         []string
	Areas          []string
	ShiftAvailable bool
	Load           int
	Role           string
}

type MemoryStore struct {
	mu            sync.RWMutex
	users         map[string]User
	usersByPhone  map[string]string
	services      map[string]Service
	slots         map[string]Slot
	orders        map[string]domain.Order
	events        []domain.OrderEvent
	audits        []AuditLog
	idempotencies map[string]string
	techs         map[string]Technician
	addresses     map[string][]Address
	reviews       []Review
	proofs        map[string][]Proof
	persistence   Persistence
}

type AuditLog struct {
	ID        string    `json:"id"`
	ActorID   string    `json:"actorId"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Result    string    `json:"result"`
	CreatedAt time.Time `json:"createdAt"`
}

// Persistence mirrors business writes to a durable store when one is configured.
type Persistence interface {
	PersistOrder(domain.Order, string) error
	PersistOrderEvent(domain.OrderEvent) error
	PersistAudit(AuditLog) error
}

type ReviewPersistence interface{ PersistReview(Review) error }
type ProofPersistence interface{ PersistProof(Proof) error }

// PersistenceSnapshot contains durable records used to rebuild the in-memory read model.
// The snapshot is applied atomically by Restore and never calls the write persistence hooks.
type PersistenceSnapshot struct {
	Orders          []domain.Order
	IdempotencyKeys map[string]string
	Events          []domain.OrderEvent
	Audits          []AuditLog
	Reviews         []Review
	Proofs          []Proof
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users: map[string]User{}, usersByPhone: map[string]string{}, services: map[string]Service{},
		slots: map[string]Slot{}, orders: map[string]domain.Order{}, idempotencies: map[string]string{}, techs: map[string]Technician{},
		addresses: map[string][]Address{},
		proofs:    map[string][]Proof{},
	}
}

// Restore replaces durable portions of the read model after a successful database load.
// Callers should only invoke it with a complete snapshot; a failed load must not be applied.
func (s *MemoryStore) Restore(snapshot PersistenceSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders = make(map[string]domain.Order, len(snapshot.Orders))
	for _, order := range snapshot.Orders {
		s.orders[order.ID] = order
	}
	s.idempotencies = make(map[string]string, len(snapshot.IdempotencyKeys))
	for key, orderID := range snapshot.IdempotencyKeys {
		if _, ok := s.orders[orderID]; ok && key != "" {
			s.idempotencies[key] = orderID
		}
	}
	s.events = append([]domain.OrderEvent(nil), snapshot.Events...)
	s.audits = append([]AuditLog(nil), snapshot.Audits...)
	s.reviews = append([]Review(nil), snapshot.Reviews...)
	s.proofs = make(map[string][]Proof)
	for _, proof := range snapshot.Proofs {
		s.proofs[proof.OrderID] = append(s.proofs[proof.OrderID], proof)
	}
}

func (s *MemoryStore) SetPersistence(p Persistence) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.persistence = p
}

func (s *MemoryStore) SeedUser(user User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[user.ID] = user
	s.usersByPhone[user.Phone] = user.ID
}

func (s *MemoryStore) UserByPhone(phone string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.usersByPhone[phone]
	if !ok {
		return User{}, ErrNotFound
	}
	u, ok := s.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (s *MemoryStore) UserByID(id string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (s *MemoryStore) SeedService(service Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services[service.ID] = service
}
func (s *MemoryStore) ServiceByID(id string) (Service, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.services[id]
	if !ok {
		return Service{}, ErrNotFound
	}
	return v, nil
}
func (s *MemoryStore) Services() []Service {
	s.mu.RLock()
	defer s.mu.RUnlock()
	values := make([]Service, 0, len(s.services))
	for _, service := range s.services {
		values = append(values, service)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values
}
func (s *MemoryStore) SeedSlot(slot Slot) { s.mu.Lock(); defer s.mu.Unlock(); s.slots[slot.ID] = slot }
func (s *MemoryStore) Slots(serviceID, date string) []Slot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	values := make([]Slot, 0)
	for _, slot := range s.slots {
		if (serviceID == "" || slot.ServiceID == serviceID) && (date == "" || slot.Date == date) {
			values = append(values, slot)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].StartsAt.Before(values[j].StartsAt) })
	return values
}

// ReserveSlot atomically increments slot usage and returns false when capacity is exhausted.
func (s *MemoryStore) ReserveSlot(slotID, serviceID, date string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	slot, ok := s.slots[slotID]
	if !ok || slot.ServiceID != serviceID || slot.Date != date || slot.Used >= slot.Capacity {
		return false
	}
	slot.Used++
	s.slots[slotID] = slot
	return true
}

func (s *MemoryStore) ReleaseSlot(slotID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if slot, ok := s.slots[slotID]; ok && slot.Used > 0 {
		slot.Used--
		s.slots[slotID] = slot
	}
}

func (s *MemoryStore) IdempotentOrder(key, fingerprint string) (domain.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.idempotencies[key]
	if !ok {
		return domain.Order{}, ErrNotFound
	}
	order, ok := s.orders[id]
	if !ok {
		return domain.Order{}, ErrNotFound
	}
	if order.UserID+"|"+order.ServiceID+"|"+order.Date+"|"+order.SlotID != fingerprint {
		return domain.Order{}, ErrIdempotencyConflict
	}
	return order, nil
}

func (s *MemoryStore) SaveOrder(order domain.Order, idempotencyKey string) {
	s.mu.Lock()
	s.orders[order.ID] = order
	if idempotencyKey != "" {
		s.idempotencies[idempotencyKey] = order.ID
	}
	event := domain.OrderEvent{ID: order.ID + "-created", OrderID: order.ID, To: order.State, ActorID: order.UserID, CreatedAt: order.CreatedAt}
	s.events = append(s.events, event)
	persistence := s.persistence
	s.mu.Unlock()
	if persistence != nil {
		if err := persistence.PersistOrder(order, idempotencyKey); err != nil {
			slog.Error("持久化订单失败", "orderId", order.ID, "error", err)
		}
		if err := persistence.PersistOrderEvent(event); err != nil {
			slog.Error("持久化订单事件失败", "orderId", order.ID, "error", err)
		}
	}
}

func (s *MemoryStore) OrderByID(id string) (domain.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.orders[id]
	if !ok {
		return domain.Order{}, ErrNotFound
	}
	return o, nil
}
func (s *MemoryStore) UpdateOrder(order domain.Order, event domain.OrderEvent) {
	s.mu.Lock()
	s.orders[order.ID] = order
	s.events = append(s.events, event)
	persistence := s.persistence
	s.mu.Unlock()
	if persistence != nil {
		if err := persistence.PersistOrder(order, ""); err != nil {
			slog.Error("持久化订单更新失败", "orderId", order.ID, "error", err)
		}
		if err := persistence.PersistOrderEvent(event); err != nil {
			slog.Error("持久化订单事件失败", "orderId", order.ID, "error", err)
		}
	}
}
func (s *MemoryStore) OrderCount() int { s.mu.RLock(); defer s.mu.RUnlock(); return len(s.orders) }
func (s *MemoryStore) Orders() []domain.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	values := make([]domain.Order, 0, len(s.orders))
	for _, order := range s.orders {
		values = append(values, order)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].CreatedAt.After(values[j].CreatedAt) })
	return values
}
func (s *MemoryStore) Events() []domain.OrderEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.OrderEvent(nil), s.events...)
}

func (s *MemoryStore) SeedTechnician(t Technician) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.techs[t.ID] = t
}
func (s *MemoryStore) Technicians() []Technician {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Technician, 0, len(s.techs))
	for _, t := range s.techs {
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
func (s *MemoryStore) AddAudit(log AuditLog) {
	s.mu.Lock()
	s.audits = append(s.audits, log)
	persistence := s.persistence
	s.mu.Unlock()
	if persistence != nil {
		if err := persistence.PersistAudit(log); err != nil {
			slog.Error("持久化审计日志失败", "auditId", log.ID, "error", err)
		}
	}
}
func (s *MemoryStore) Audits() []AuditLog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]AuditLog(nil), s.audits...)
}

func (s *MemoryStore) AddAddress(address Address) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addresses[address.UserID] = append(s.addresses[address.UserID], address)
}
func (s *MemoryStore) Addresses(userID string) []Address {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Address(nil), s.addresses[userID]...)
}

func (s *MemoryStore) AddReview(review Review) {
	s.mu.Lock()
	s.reviews = append(s.reviews, review)
	persistence := s.persistence
	s.mu.Unlock()
	if saver, ok := persistence.(ReviewPersistence); ok {
		if err := saver.PersistReview(review); err != nil {
			slog.Error("持久化评价失败", "reviewId", review.ID, "error", err)
		}
	}
}
func (s *MemoryStore) Reviews() []Review {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Review(nil), s.reviews...)
}
func (s *MemoryStore) AddProof(proof Proof) {
	s.mu.Lock()
	s.proofs[proof.OrderID] = append(s.proofs[proof.OrderID], proof)
	persistence := s.persistence
	s.mu.Unlock()
	if saver, ok := persistence.(ProofPersistence); ok {
		if err := saver.PersistProof(proof); err != nil {
			slog.Error("持久化履约凭证失败", "proofId", proof.ID, "error", err)
		}
	}
}
func (s *MemoryStore) Proofs(orderID string) []Proof {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Proof(nil), s.proofs[orderID]...)
}
