package equity

import "testing"

func TestValidatePriceRejectsAmbiguousSymbol(t *testing.T) {
	def := IndexDef{ID: "sh000001", MinPrice: 1000, MaxPrice: 10000}
	if err := validatePrice(def, 10.99); err == nil {
		t.Fatal("expected suspicious stock-like price to be rejected")
	}
	if err := validatePrice(def, 3200); err != nil {
		t.Fatalf("valid index price rejected: %v", err)
	}
}
