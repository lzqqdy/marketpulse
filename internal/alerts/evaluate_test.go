package alerts

import "testing"

func TestIsConditionMet_Type1(t *testing.T) {
	target := 100.0
	params := RuleParams{Target: &target}
	tests := []struct {
		price float64
		want  bool
	}{
		{99.9, false},
		{100, true},
		{101, true},
	}
	for _, tc := range tests {
		got, _ := IsConditionMet(1, tc.price, params, 0, false)
		if got != tc.want {
			t.Fatalf("type1 price=%v want=%v got=%v", tc.price, tc.want, got)
		}
	}
}

func TestIsConditionMet_Type2(t *testing.T) {
	target := 50.0
	params := RuleParams{Target: &target}
	got, _ := IsConditionMet(2, 49.9, params, 0, false)
	if !got {
		t.Fatal("type2 should trigger at 49.9")
	}
	got, _ = IsConditionMet(2, 50.1, params, 0, false)
	if got {
		t.Fatal("type2 should not trigger above target")
	}
}

func TestIsConditionMet_Type3(t *testing.T) {
	upper, lower := 110.0, 90.0
	params := RuleParams{Upper: &upper, Lower: &lower}
	if met, _ := IsConditionMet(3, 95, params, 0, false); met {
		t.Fatal("inside range should not trigger")
	}
	if met, _ := IsConditionMet(3, 110, params, 0, false); !met {
		t.Fatal("upper bound should trigger")
	}
	if met, _ := IsConditionMet(3, 90, params, 0, false); !met {
		t.Fatal("lower bound should trigger")
	}
}

func TestIsConditionMet_Type4(t *testing.T) {
	upper, lower := 105.0, 95.0
	params := RuleParams{Upper: &upper, Lower: &lower}
	if met, _ := IsConditionMet(4, 100, params, 0, false); met {
		t.Fatal("inside ampl band should not trigger")
	}
	if met, _ := IsConditionMet(4, 105, params, 0, false); !met {
		t.Fatal("upper ampl should trigger")
	}
}

func TestIsConditionMet_Type5(t *testing.T) {
	rapid := 5.0
	params := RuleParams{RapidChg: &rapid}
	if met, _ := IsConditionMet(5, 100, params, 4.9, true); met {
		t.Fatal("below rapid threshold should not trigger")
	}
	if met, _ := IsConditionMet(5, 100, params, 5.0, true); !met {
		t.Fatal("at rapid threshold should trigger")
	}
	if met, _ := IsConditionMet(5, 100, params, 10, false); met {
		t.Fatal("window not ready should not trigger")
	}
}

func TestConditionAlreadyMetAtCreate_Type5WindowNotReady(t *testing.T) {
	rapid := 3.0
	params := RuleParams{RapidChg: &rapid}
	if ConditionAlreadyMetAtCreate(5, 100, params, 10, false) {
		t.Fatal("insufficient window should allow create")
	}
}

func TestConditionAlreadyMetAtCreate(t *testing.T) {
	target := 100.0
	params := RuleParams{Target: &target}
	if !ConditionAlreadyMetAtCreate(1, 100, params, 0, false) {
		t.Fatal("type1 at target should reject create")
	}
	if ConditionAlreadyMetAtCreate(1, 99, params, 0, false) {
		t.Fatal("type1 below target should allow create")
	}
}

func TestBuildBounds_Type3(t *testing.T) {
	rng := 10.0
	params, err := BuildBounds(3, 100, RuleParams{Range: &rng})
	if err != nil {
		t.Fatal(err)
	}
	if *params.Upper != 110 || *params.Lower != 90 {
		t.Fatalf("unexpected bounds %+v", params)
	}
}

func TestBuildBounds_Type4(t *testing.T) {
	ampl := 5.0
	params, err := BuildBounds(4, 200, RuleParams{Ampl: &ampl})
	if err != nil {
		t.Fatal(err)
	}
	if *params.Upper != 210 || *params.Lower != 190 {
		t.Fatalf("unexpected bounds %+v", params)
	}
}
