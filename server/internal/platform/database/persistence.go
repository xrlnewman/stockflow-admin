package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/xrlnewman/stockflow-admin/server/internal/domain"
	"github.com/xrlnewman/stockflow-admin/server/internal/platform/store"
)

// SQLPersistence stores order facts and audit events in MySQL 8.4.
type SQLPersistence struct{ db *sql.DB }

func NewSQLPersistence(db *sql.DB) *SQLPersistence { return &SQLPersistence{db: db} }

// Load reads a consistent durable snapshot for rebuilding the in-memory read model.
// A partial snapshot is never returned without an error, so callers can safely avoid applying it.
func (p *SQLPersistence) Load(ctx context.Context) (store.PersistenceSnapshot, error) {
	if p == nil || p.db == nil {
		return store.PersistenceSnapshot{}, fmt.Errorf("mysql persistence is not configured")
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return store.PersistenceSnapshot{}, fmt.Errorf("begin persistence load: %w", err)
	}
	defer tx.Rollback()
	var snapshot store.PersistenceSnapshot
	snapshot.IdempotencyKeys = map[string]string{}

	orders, err := tx.QueryContext(ctx, `SELECT id,user_id,service_id,address_id,service_date,slot_id,technician_id,remark,state,idempotency_key,created_at,updated_at FROM orders ORDER BY created_at,id`)
	if err != nil {
		return store.PersistenceSnapshot{}, fmt.Errorf("load orders: %w", err)
	}
	for orders.Next() {
		var order domain.Order
		var serviceDate time.Time
		var technicianID, remark, idempotencyKey sql.NullString
		var state string
		if err := orders.Scan(&order.ID, &order.UserID, &order.ServiceID, &order.AddressID, &serviceDate, &order.SlotID, &technicianID, &remark, &state, &idempotencyKey, &order.CreatedAt, &order.UpdatedAt); err != nil {
			orders.Close()
			return store.PersistenceSnapshot{}, fmt.Errorf("scan orders: %w", err)
		}
		order.Date = serviceDate.Format("2006-01-02")
		order.TechnicianID = nullableString(technicianID)
		order.Remark = nullableString(remark)
		order.State = domain.OrderState(state)
		snapshot.Orders = append(snapshot.Orders, order)
		if key := nullableString(idempotencyKey); key != "" {
			snapshot.IdempotencyKeys[key] = order.ID
		}
	}
	if err := orders.Err(); err != nil {
		orders.Close()
		return store.PersistenceSnapshot{}, fmt.Errorf("read orders: %w", err)
	}
	if err := orders.Close(); err != nil {
		return store.PersistenceSnapshot{}, fmt.Errorf("close orders: %w", err)
	}

	events, err := tx.QueryContext(ctx, `SELECT id,order_id,from_state,to_state,actor_id,created_at FROM order_events ORDER BY created_at,id`)
	if err != nil {
		return store.PersistenceSnapshot{}, fmt.Errorf("load order events: %w", err)
	}
	for events.Next() {
		var event domain.OrderEvent
		var fromState sql.NullString
		var toState string
		if err := events.Scan(&event.ID, &event.OrderID, &fromState, &toState, &event.ActorID, &event.CreatedAt); err != nil {
			events.Close()
			return store.PersistenceSnapshot{}, fmt.Errorf("scan order events: %w", err)
		}
		event.From = domain.OrderState(nullableString(fromState))
		event.To = domain.OrderState(toState)
		snapshot.Events = append(snapshot.Events, event)
	}
	if err := events.Err(); err != nil {
		events.Close()
		return store.PersistenceSnapshot{}, fmt.Errorf("read order events: %w", err)
	}
	if err := events.Close(); err != nil {
		return store.PersistenceSnapshot{}, fmt.Errorf("close order events: %w", err)
	}

	audits, err := tx.QueryContext(ctx, `SELECT id,actor_id,action,resource,result,created_at FROM audit_logs ORDER BY created_at,id`)
	if err != nil {
		return store.PersistenceSnapshot{}, fmt.Errorf("load audit logs: %w", err)
	}
	for audits.Next() {
		var audit store.AuditLog
		if err := audits.Scan(&audit.ID, &audit.ActorID, &audit.Action, &audit.Resource, &audit.Result, &audit.CreatedAt); err != nil {
			audits.Close()
			return store.PersistenceSnapshot{}, fmt.Errorf("scan audit logs: %w", err)
		}
		snapshot.Audits = append(snapshot.Audits, audit)
	}
	if err := audits.Err(); err != nil {
		audits.Close()
		return store.PersistenceSnapshot{}, fmt.Errorf("read audit logs: %w", err)
	}
	if err := audits.Close(); err != nil {
		return store.PersistenceSnapshot{}, fmt.Errorf("close audit logs: %w", err)
	}

	reviews, err := tx.QueryContext(ctx, `SELECT id,order_id,user_id,rating,content,created_at FROM reviews ORDER BY created_at,id`)
	if err != nil {
		return store.PersistenceSnapshot{}, fmt.Errorf("load reviews: %w", err)
	}
	for reviews.Next() {
		var review store.Review
		if err := reviews.Scan(&review.ID, &review.OrderID, &review.UserID, &review.Rating, &review.Content, &review.CreatedAt); err != nil {
			reviews.Close()
			return store.PersistenceSnapshot{}, fmt.Errorf("scan reviews: %w", err)
		}
		snapshot.Reviews = append(snapshot.Reviews, review)
	}
	if err := reviews.Err(); err != nil {
		reviews.Close()
		return store.PersistenceSnapshot{}, fmt.Errorf("read reviews: %w", err)
	}
	if err := reviews.Close(); err != nil {
		return store.PersistenceSnapshot{}, fmt.Errorf("close reviews: %w", err)
	}

	proofs, err := tx.QueryContext(ctx, `SELECT id,order_id,kind,filename,note,created_at FROM work_proofs ORDER BY created_at,id`)
	if err != nil {
		return store.PersistenceSnapshot{}, fmt.Errorf("load work proofs: %w", err)
	}
	for proofs.Next() {
		var proof store.Proof
		var note sql.NullString
		if err := proofs.Scan(&proof.ID, &proof.OrderID, &proof.Kind, &proof.Filename, &note, &proof.CreatedAt); err != nil {
			proofs.Close()
			return store.PersistenceSnapshot{}, fmt.Errorf("scan work proofs: %w", err)
		}
		proof.Note = nullableString(note)
		snapshot.Proofs = append(snapshot.Proofs, proof)
	}
	if err := proofs.Err(); err != nil {
		proofs.Close()
		return store.PersistenceSnapshot{}, fmt.Errorf("read work proofs: %w", err)
	}
	if err := proofs.Close(); err != nil {
		return store.PersistenceSnapshot{}, fmt.Errorf("close work proofs: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return store.PersistenceSnapshot{}, fmt.Errorf("commit persistence load: %w", err)
	}

	return snapshot, nil
}

func (p *SQLPersistence) PersistOrder(order domain.Order, idempotencyKey string) error {
	if p == nil || p.db == nil {
		return fmt.Errorf("mysql persistence is not configured")
	}
	_, err := p.db.Exec(`INSERT INTO orders (id,user_id,service_id,address_id,service_date,slot_id,technician_id,remark,state,idempotency_key,created_at,updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
ON DUPLICATE KEY UPDATE technician_id=VALUES(technician_id), remark=VALUES(remark), state=VALUES(state), updated_at=VALUES(updated_at)`,
		order.ID, order.UserID, order.ServiceID, order.AddressID, order.Date, order.SlotID, nullable(order.TechnicianID), nullable(order.Remark), order.State, nullable(idempotencyKey), order.CreatedAt, order.UpdatedAt)
	return err
}

func (p *SQLPersistence) PersistOrderEvent(event domain.OrderEvent) error {
	if p == nil || p.db == nil {
		return fmt.Errorf("mysql persistence is not configured")
	}
	_, err := p.db.Exec(`INSERT INTO order_events (id,order_id,from_state,to_state,actor_id,created_at) VALUES (?,?,?,?,?,?)`, event.ID, event.OrderID, nullable(string(event.From)), string(event.To), event.ActorID, event.CreatedAt)
	return err
}

func (p *SQLPersistence) PersistAudit(log store.AuditLog) error {
	if p == nil || p.db == nil {
		return fmt.Errorf("mysql persistence is not configured")
	}
	_, err := p.db.Exec(`INSERT INTO audit_logs (id,actor_id,action,resource,result,created_at) VALUES (?,?,?,?,?,?)`, log.ID, log.ActorID, log.Action, log.Resource, log.Result, log.CreatedAt)
	return err
}

func (p *SQLPersistence) PersistReview(review store.Review) error {
	if p == nil || p.db == nil {
		return fmt.Errorf("mysql persistence is not configured")
	}
	_, err := p.db.Exec(`INSERT INTO reviews (id,order_id,user_id,rating,content,created_at) VALUES (?,?,?,?,?,?)`, review.ID, review.OrderID, review.UserID, review.Rating, review.Content, review.CreatedAt)
	return err
}

func (p *SQLPersistence) PersistProof(proof store.Proof) error {
	if p == nil || p.db == nil {
		return fmt.Errorf("mysql persistence is not configured")
	}
	_, err := p.db.Exec(`INSERT INTO work_proofs (id,order_id,kind,filename,note,created_at) VALUES (?,?,?,?,?,?)`, proof.ID, proof.OrderID, proof.Kind, proof.Filename, nullable(proof.Note), proof.CreatedAt)
	return err
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
