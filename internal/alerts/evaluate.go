package alerts

import "math"

// IsConditionMet reports whether a rule type is satisfied at the given price/window amplitude.
// triggerValue is the observed value written to deliveries (price or amplitude %).
func IsConditionMet(ruleType int, price float64, params RuleParams, windowAmpPct float64, windowReady bool) (bool, float64) {
	switch ruleType {
	case 1:
		if params.Target != nil && price >= *params.Target {
			return true, price
		}
	case 2:
		if params.Target != nil && price <= *params.Target {
			return true, price
		}
	case 3, 4:
		if params.Upper != nil && price >= *params.Upper {
			return true, price
		}
		if params.Lower != nil && price <= *params.Lower {
			return true, price
		}
	case 5:
		if !windowReady {
			return false, 0
		}
		if params.RapidChg != nil && windowAmpPct >= *params.RapidChg {
			return true, windowAmpPct
		}
	}
	return false, 0
}

// ConditionAlreadyMetAtCreate mirrors create-time rejection rules from the spec.
func ConditionAlreadyMetAtCreate(ruleType int, price float64, params RuleParams, windowAmpPct float64, windowReady bool) bool {
	met, _ := IsConditionMet(ruleType, price, params, windowAmpPct, windowReady)
	if ruleType == 5 && !windowReady {
		return false
	}
	return met
}

// BuildBounds fills upper/lower for range/ampl types at creation time.
func BuildBounds(ruleType int, setPrice float64, params RuleParams) (RuleParams, error) {
	out := params
	switch ruleType {
	case 3:
		if params.Range == nil || *params.Range <= 0 {
			return RuleParams{}, ErrInvalidParams
		}
		r := *params.Range
		upper := setPrice + r
		lower := setPrice - r
		out.Upper = &upper
		out.Lower = &lower
	case 4:
		if params.Ampl == nil || *params.Ampl <= 0 {
			return RuleParams{}, ErrInvalidParams
		}
		pct := *params.Ampl / 100
		upper := setPrice * (1 + pct)
		lower := setPrice * (1 - pct)
		out.Upper = &upper
		out.Lower = &lower
	}
	return out, nil
}

// ValidateCreateParams checks rule_type specific inputs before bounds are built.
func ValidateCreateParams(ruleType int, params RuleParams) error {
	switch ruleType {
	case 1, 2:
		if params.Target == nil || *params.Target <= 0 || math.IsNaN(*params.Target) {
			return ErrInvalidParams
		}
	case 3:
		if params.Range == nil || *params.Range <= 0 {
			return ErrInvalidParams
		}
	case 4:
		if params.Ampl == nil || *params.Ampl <= 0 {
			return ErrInvalidParams
		}
	case 5:
		if params.RapidChg == nil || *params.RapidChg <= 0 {
			return ErrInvalidParams
		}
	default:
		return ErrInvalidParams
	}
	return nil
}
