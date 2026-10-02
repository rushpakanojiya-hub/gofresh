package services

import "strconv"

// OrderPushData is the FCM data payload that lets the customer app open the right order on tap.
func OrderPushData(orderID uint) map[string]string {
	return map[string]string{"type": "order_status", "order_id": strconv.FormatUint(uint64(orderID), 10)}
}
