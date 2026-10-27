package handlers

import "testing"

// BUG-11: purchase order status state machine.
func TestPOTransitionRules(t *testing.T) {
    allowed := [][2]string{
        {"draft", "sent"}, {"draft", "confirmed"}, {"draft", "cancelled"},
        {"sent", "confirmed"}, {"sent", "cancelled"},
        {"confirmed", "cancelled"},
    }
    for _, p := range allowed {
        if !poTransitionAllowed(p[0], p[1]) {
            t.Errorf("%s -> %s should be allowed", p[0], p[1])
        }
    }

    blocked := [][2]string{
        {"received", "cancelled"}, {"received", "draft"}, {"received", "sent"},
        {"partially_received", "cancelled"}, {"partially_received", "draft"},
        {"cancelled", "draft"}, {"cancelled", "sent"},
        {"sent", "draft"}, {"confirmed", "draft"},
        {"draft", "received"}, {"sent", "received"}, {"draft", "partially_received"},
        {"draft", "draft"}, {"", "sent"},
    }
    for _, p := range blocked {
        if poTransitionAllowed(p[0], p[1]) {
            t.Errorf("%s -> %s should be blocked", p[0], p[1])
        }
    }
}