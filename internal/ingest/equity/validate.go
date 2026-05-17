package equity

import "fmt"

func validatePrice(def IndexDef, price float64) error {
	if price <= 0 {
		return fmt.Errorf("%s: empty quote", def.ID)
	}
	if def.MinPrice > 0 && price < def.MinPrice {
		return fmt.Errorf("%s: suspicious price %.4f below %.4f", def.ID, price, def.MinPrice)
	}
	if def.MaxPrice > 0 && price > def.MaxPrice {
		return fmt.Errorf("%s: suspicious price %.4f above %.4f", def.ID, price, def.MaxPrice)
	}
	return nil
}
