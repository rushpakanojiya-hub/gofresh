package handlers

import "sync"

// BUG-22: per-order mutex so two concurrent RequestReturn calls for the same
// order cannot both pass the alreadyReturned check and create duplicate
// approved returns. In-process only: protects a single API instance.
var returnOrderLocks sync.Map

func lockReturnOrder(orderID string) func() {
	v, _ := returnOrderLocks.LoadOrStore(orderID, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}
