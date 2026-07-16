package domain

import "testing"

func TestOrderStateTransitionRejectsSkippingDispatch(t *testing.T) {
	if err := ValidateTransition(OrderPendingConfirmation, OrderServing); err == nil {
		t.Fatal("expected invalid transition error")
	}
}

func TestOrderStateTransitionAllowsHappyPath(t *testing.T) {
	path := []OrderState{OrderPendingConfirmation, OrderPendingDispatch, OrderAssigned, OrderEnRoute, OrderServing, OrderPendingCustomerConfirmation, OrderCompleted}
	for i := 1; i < len(path); i++ {
		if err := ValidateTransition(path[i-1], path[i]); err != nil {
			t.Fatalf("transition %s -> %s: %v", path[i-1], path[i], err)
		}
	}
}
