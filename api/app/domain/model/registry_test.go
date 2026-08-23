package model

import "testing"

// TestAllModelsIncludesDeliveryHistory pins the registration of every model the
// schema needs.
//
// A model missing from AllModels fails silently and nowhere near the cause:
// AutoMigrate simply never creates the table, and the first anyone hears of it
// is a "no such table" from whichever query happens to run first.
func TestAllModelsIncludesDeliveryHistory(t *testing.T) {
	for _, m := range AllModels() {
		if _, ok := m.(*DeliveryHistory); ok {
			return
		}
	}
	t.Fatal("AllModels() does not include *DeliveryHistory; the table would never be created")
}
