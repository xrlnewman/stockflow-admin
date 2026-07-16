package order

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xrlnewman/stockflow-admin/server/internal/domain"
	"github.com/xrlnewman/stockflow-admin/server/internal/platform/store"
)

var (
	ErrSlotUnavailable = errors.New("slot unavailable")
	ErrInvalidInput    = errors.New("invalid order input")
	ErrOrderNotFound   = errors.New("order not found")
)

type Service = store.Service
type Slot = store.Slot
type MemoryStore = store.MemoryStore

func NewMemoryStore() *store.MemoryStore { return store.NewMemoryStore() }

type CreateInput struct {
	UserID         string
	ServiceID      string
	AddressID      string
	Date           string
	SlotID         string
	Remark         string
	IdempotencyKey string
}

type OrderService struct {
	store    *store.MemoryStore
	locker   Locker
	createMu sync.Mutex
}

type Locker interface {
	Acquire(context.Context, string, time.Duration) (bool, error)
}

func NewService(s *store.MemoryStore, locker Locker) *OrderService {
	return &OrderService{store: s, locker: locker}
}

func (s *OrderService) Create(ctx context.Context, in CreateInput) (domain.Order, error) {
	s.createMu.Lock()
	defer s.createMu.Unlock()
	if in.UserID == "" || in.ServiceID == "" || in.AddressID == "" || in.Date == "" || in.SlotID == "" {
		return domain.Order{}, ErrInvalidInput
	}
	fingerprint := fmt.Sprintf("%s|%s|%s|%s", in.UserID, in.ServiceID, in.Date, in.SlotID)
	if in.IdempotencyKey != "" {
		if existing, err := s.store.IdempotentOrder(in.IdempotencyKey, fingerprint); err == nil {
			return existing, nil
		} else if !errors.Is(err, store.ErrNotFound) {
			return domain.Order{}, err
		}
	}
	if s.locker != nil {
		ok, err := s.locker.Acquire(ctx, "stockflow:slot:"+in.SlotID, 8*time.Second)
		if err != nil {
			return domain.Order{}, fmt.Errorf("acquire slot lock: %w", err)
		}
		if !ok {
			return domain.Order{}, ErrSlotUnavailable
		}
	}
	if !s.store.ReserveSlot(in.SlotID, in.ServiceID, in.Date) {
		return domain.Order{}, ErrSlotUnavailable
	}
	now := time.Now().UTC()
	order := domain.Order{ID: uuid.NewString(), UserID: in.UserID, ServiceID: in.ServiceID, AddressID: in.AddressID, Date: in.Date, SlotID: in.SlotID, Remark: in.Remark, State: domain.OrderPendingConfirmation, CreatedAt: now, UpdatedAt: now}
	s.store.SaveOrder(order, in.IdempotencyKey)
	return order, nil
}

func (s *OrderService) Transition(_ context.Context, orderID, actorID string, to domain.OrderState) (domain.Order, error) {
	current, err := s.store.OrderByID(orderID)
	if err != nil {
		return domain.Order{}, ErrOrderNotFound
	}
	if err := domain.ValidateTransition(current.State, to); err != nil {
		return domain.Order{}, err
	}
	from := current.State
	current.State = to
	current.UpdatedAt = time.Now().UTC()
	s.store.UpdateOrder(current, domain.OrderEvent{ID: uuid.NewString(), OrderID: orderID, From: from, To: to, ActorID: actorID, CreatedAt: current.UpdatedAt})
	return current, nil
}

func (s *OrderService) Assign(_ context.Context, orderID, actorID, technicianID string) (domain.Order, error) {
	current, err := s.store.OrderByID(orderID)
	if err != nil {
		return domain.Order{}, ErrOrderNotFound
	}
	if err := domain.ValidateTransition(current.State, domain.OrderPendingDispatch); current.State == domain.OrderPendingConfirmation && err == nil {
		current, _ = s.Transition(context.Background(), orderID, actorID, domain.OrderPendingDispatch)
	}
	current, err = s.store.OrderByID(orderID)
	if err != nil {
		return domain.Order{}, ErrOrderNotFound
	}
	if err = domain.ValidateTransition(current.State, domain.OrderAssigned); err != nil {
		return domain.Order{}, err
	}
	from := current.State
	current.TechnicianID = technicianID
	current.State = domain.OrderAssigned
	current.UpdatedAt = time.Now().UTC()
	s.store.UpdateOrder(current, domain.OrderEvent{ID: uuid.NewString(), OrderID: orderID, From: from, To: domain.OrderAssigned, ActorID: actorID, CreatedAt: current.UpdatedAt})
	return current, nil
}
