package domain

import (
	"errors"
	"time"
)

type OrderState string

const (
	OrderPendingConfirmation         OrderState = "pending_confirmation"
	OrderPendingDispatch             OrderState = "pending_dispatch"
	OrderAssigned                    OrderState = "assigned"
	OrderEnRoute                     OrderState = "en_route"
	OrderServing                     OrderState = "serving"
	OrderPendingCustomerConfirmation OrderState = "pending_customer_confirmation"
	OrderCompleted                   OrderState = "completed"
	OrderCancelled                   OrderState = "cancelled"
)

var ErrOrderStateInvalid = errors.New("order state transition is invalid")

type Order struct {
	ID           string     `json:"id"`
	UserID       string     `json:"userId"`
	ServiceID    string     `json:"serviceId"`
	AddressID    string     `json:"addressId"`
	Date         string     `json:"date"`
	SlotID       string     `json:"slotId"`
	Remark       string     `json:"remark"`
	TechnicianID string     `json:"technicianId,omitempty"`
	State        OrderState `json:"state"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type OrderEvent struct {
	ID        string     `json:"id"`
	OrderID   string     `json:"orderId"`
	From      OrderState `json:"from"`
	To        OrderState `json:"to"`
	ActorID   string     `json:"actorId"`
	CreatedAt time.Time  `json:"createdAt"`
}

func ValidateTransition(from, to OrderState) error {
	if from == to {
		return ErrOrderStateInvalid
	}
	allowed := map[OrderState][]OrderState{
		OrderPendingConfirmation:         {OrderPendingDispatch, OrderCancelled},
		OrderPendingDispatch:             {OrderAssigned, OrderCancelled},
		OrderAssigned:                    {OrderEnRoute, OrderCancelled},
		OrderEnRoute:                     {OrderServing},
		OrderServing:                     {OrderPendingCustomerConfirmation},
		OrderPendingCustomerConfirmation: {OrderCompleted},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return nil
		}
	}
	return ErrOrderStateInvalid
}
