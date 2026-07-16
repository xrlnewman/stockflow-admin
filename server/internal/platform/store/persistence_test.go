package store

import (
	"testing"
	"time"

	"github.com/xrlnewman/stockflow-admin/server/internal/domain"
)

type recordingPersistence struct{ orders, events, audits int }

func (p *recordingPersistence) PersistOrder(domain.Order, string) error   { p.orders++; return nil }
func (p *recordingPersistence) PersistOrderEvent(domain.OrderEvent) error { p.events++; return nil }
func (p *recordingPersistence) PersistAudit(AuditLog) error               { p.audits++; return nil }

func TestMemoryStoreForwardsWritesToPersistence(t *testing.T) {
	st := NewMemoryStore()
	recorder := &recordingPersistence{}
	st.SetPersistence(recorder)
	order := domain.Order{ID: "order-1", UserID: "user-1", State: domain.OrderPendingConfirmation}
	st.SaveOrder(order, "idem-1")
	st.UpdateOrder(order, domain.OrderEvent{ID: "event-1", OrderID: order.ID, To: order.State})
	st.AddAudit(AuditLog{ID: "audit-1", ActorID: "user-1", Action: "test", Resource: order.ID, Result: "success"})
	if recorder.orders != 2 || recorder.events != 2 || recorder.audits != 1 {
		t.Fatalf("persistence writes: orders=%d events=%d audits=%d", recorder.orders, recorder.events, recorder.audits)
	}
}

func TestMemoryStoreRestoresPersistedSnapshot(t *testing.T) {
	createdAt := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	st := NewMemoryStore()
	st.Restore(PersistenceSnapshot{
		Orders: []domain.Order{{
			ID: "order-restored", UserID: "user-1", ServiceID: "service-1", AddressID: "address-1",
			Date: "2026-07-16", SlotID: "slot-1", State: domain.OrderAssigned, CreatedAt: createdAt, UpdatedAt: createdAt,
		}},
		IdempotencyKeys: map[string]string{"idem-restored": "order-restored"},
		Events:          []domain.OrderEvent{{ID: "event-restored", OrderID: "order-restored", To: domain.OrderAssigned, ActorID: "admin-1", CreatedAt: createdAt}},
		Audits:          []AuditLog{{ID: "audit-restored", ActorID: "admin-1", Action: "assign", Resource: "order-restored", Result: "success", CreatedAt: createdAt}},
	})

	order, err := st.OrderByID("order-restored")
	if err != nil || order.State != domain.OrderAssigned {
		t.Fatalf("restored order = %#v, err = %v", order, err)
	}
	if _, err := st.IdempotentOrder("idem-restored", "user-1|service-1|2026-07-16|slot-1"); err != nil {
		t.Fatalf("restored idempotency key: %v", err)
	}
	if got := len(st.Events()); got != 1 {
		t.Fatalf("restored events = %d, want 1", got)
	}
	if got := len(st.Audits()); got != 1 {
		t.Fatalf("restored audits = %d, want 1", got)
	}
}
