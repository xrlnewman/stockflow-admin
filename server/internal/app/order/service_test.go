package order

import (
	"context"
	"testing"
	"time"

	"github.com/xrlnewman/stockflow-admin/server/internal/domain"
)

func TestCreateOrderRejectsFullSlot(t *testing.T) {
	store := NewMemoryStore()
	store.SeedService(Service{ID: "svc-clean", Name: "深度保洁", SlotCapacity: 1})
	store.SeedSlot(Slot{ID: "slot-am", ServiceID: "svc-clean", Date: "2026-07-18", StartsAt: time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC), Capacity: 1, Used: 1})
	svc := NewService(store, nil)
	_, err := svc.Create(context.Background(), CreateInput{UserID: "u-1", ServiceID: "svc-clean", AddressID: "addr-1", Date: "2026-07-18", SlotID: "slot-am", IdempotencyKey: "id-1"})
	if err == nil || err != ErrSlotUnavailable {
		t.Fatalf("expected ErrSlotUnavailable, got %v", err)
	}
}

func TestCreateOrderIsIdempotent(t *testing.T) {
	store := NewMemoryStore()
	store.SeedService(Service{ID: "svc-clean", Name: "深度保洁", SlotCapacity: 1})
	store.SeedSlot(Slot{ID: "slot-am", ServiceID: "svc-clean", Date: "2026-07-18", StartsAt: time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC), Capacity: 1})
	svc := NewService(store, nil)
	in := CreateInput{UserID: "u-1", ServiceID: "svc-clean", AddressID: "addr-1", Date: "2026-07-18", SlotID: "slot-am", IdempotencyKey: "id-same"}
	first, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	second, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if first.ID != second.ID || store.OrderCount() != 1 {
		t.Fatalf("expected one order, first=%s second=%s count=%d", first.ID, second.ID, store.OrderCount())
	}
	if first.State != domain.OrderPendingConfirmation {
		t.Fatalf("unexpected initial state %s", first.State)
	}
}

func TestTransitionPersistsNewStateAndEvent(t *testing.T) {
	store := NewMemoryStore()
	order := domain.Order{ID: "order-state", UserID: "u-1", State: domain.OrderPendingConfirmation}
	store.SaveOrder(order, "")
	svc := NewService(store, nil)
	updated, err := svc.Transition(context.Background(), order.ID, "u-1", domain.OrderPendingDispatch)
	if err != nil {
		t.Fatalf("transition: %v", err)
	}
	if updated.State != domain.OrderPendingDispatch {
		t.Fatalf("returned state=%s", updated.State)
	}
	saved, err := store.OrderByID(order.ID)
	if err != nil || saved.State != domain.OrderPendingDispatch {
		t.Fatalf("saved state=%s err=%v", saved.State, err)
	}
	events := store.Events()
	if len(events) != 2 || events[1].From != domain.OrderPendingConfirmation || events[1].To != domain.OrderPendingDispatch {
		t.Fatalf("unexpected events: %#v", events)
	}
}
