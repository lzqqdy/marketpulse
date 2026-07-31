package event

import "testing"

func TestComputeScore_NormalizesMissingComponents(t *testing.T) {
	p := 80.0
	v := 60.0
	full := ComputeScore(ComponentScores{Price: &p, Volume: &v, Volatility: floatPtr(40), Liquidation: floatPtr(90)})
	partial := ComputeScore(ComponentScores{Price: &p, Volume: &v})
	if partial <= 0 || partial > 100 {
		t.Fatalf("partial score out of range: %v", partial)
	}
	// Missing components should not drag toward zero like raw zeros would.
	onlyPrice := ComputeScore(ComponentScores{Price: &p})
	if onlyPrice < 70 {
		t.Fatalf("expected price-only near 80, got %v", onlyPrice)
	}
	_ = full
}

func TestSeverityFromScore(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{10, SeverityNormal},
		{40, SeverityLow},
		{60, SeverityMedium},
		{80, SeverityHigh},
		{90, SeverityExtreme},
	}
	for _, tc := range cases {
		if got := SeverityFromScore(tc.score); got != tc.want {
			t.Fatalf("score %v: got %s want %s", tc.score, got, tc.want)
		}
	}
}

func TestScoreFromSignals_FlashComponents(t *testing.T) {
	score, sev := ScoreFromSignals([]EventSignal{
		{SignalType: SignalPriceDrop, ChangePct: -4.2},
		{SignalType: SignalVolumeSpike, Ratio: 4.8},
		{SignalType: SignalLiquidationSpike, Ratio: 9.2},
	})
	if score < 50 {
		t.Fatalf("expected meaningful score, got %v", score)
	}
	if sev == SeverityNormal {
		t.Fatalf("unexpected NORMAL severity for flash signals")
	}
}

func floatPtr(v float64) *float64 { return &v }
