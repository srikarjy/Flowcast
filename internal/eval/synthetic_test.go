package eval

import (
	"math"
	"math/rand"
	"testing"
)

func TestGenerateTrial_NoOutlier(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	trial := GenerateTrial(rng, 5, false, 0)
	if trial.TrueOutlier != "" {
		t.Fatalf("expected no true outlier, got %q", trial.TrueOutlier)
	}
	if len(trial.Values) != 5 {
		t.Fatalf("expected 5 values, got %d", len(trial.Values))
	}
}

func TestGenerateTrial_LargeEffectAlwaysDetectedByAdaptive(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	hits := 0
	const n = 200
	for i := 0; i < n; i++ {
		trial := GenerateTrial(rng, 5, true, 12) // 12 sigma: matches real WT_REP1 scale
		flagged := AdaptiveFlags(trial)
		if flagged[trial.TrueOutlier] {
			hits++
		}
	}
	if hits < n*9/10 {
		t.Fatalf("expected adaptive classifier to catch >=90%% of 12-sigma outliers, got %d/%d", hits, n)
	}
}

// TestGenerateTrial_ZeroEffectFlagRateBounded checks the adaptive
// classifier's false-positive rate on clean (no true outlier) 5-sample
// runs stays near its known small-n rate. At n=5 the modified z-score has
// a real, non-trivial false-positive rate (~20% empirically) because MAD
// is noisy with so few points — this is a documented property, not a bug,
// and this test guards against a change silently making it much worse.
func TestGenerateTrial_ZeroEffectFlagRateBounded(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	trialsWithFlags := 0
	const n = 1000
	for i := 0; i < n; i++ {
		trial := GenerateTrial(rng, 5, false, 0)
		if len(AdaptiveFlags(trial)) > 0 {
			trialsWithFlags++
		}
	}
	rate := float64(trialsWithFlags) / float64(n)
	if rate > 0.35 {
		t.Fatalf("clean-run flag rate = %.2f, expected roughly ~0.20 (n=5 small-sample MAD noise); got a much higher rate", rate)
	}
}

func TestCounts_PrecisionRecallF1(t *testing.T) {
	c := Counts{TP: 8, FP: 2, FN: 2}
	if got := c.Precision(); got != 0.8 {
		t.Errorf("precision = %.3f, want 0.8", got)
	}
	if got := c.Recall(); got != 0.8 {
		t.Errorf("recall = %.3f, want 0.8", got)
	}
	if got := c.F1(); math.Abs(got-0.8) > 1e-9 {
		t.Errorf("f1 = %.3f, want 0.8", got)
	}
}

func TestEvaluate_Deterministic(t *testing.T) {
	r1 := Evaluate(500, 123)
	r2 := Evaluate(500, 123)
	if r1.Adaptive.F1() != r2.Adaptive.F1() || r1.Fixed.F1() != r2.Fixed.F1() {
		t.Fatal("Evaluate with the same seed should be deterministic")
	}
}

func TestEvaluate_ReportRendersMethodologyLabel(t *testing.T) {
	r := Evaluate(50, 1)
	s := r.String()
	if !contains(s, "SYNTHETIC benchmark") || !contains(s, "NOT real pipeline output") {
		t.Fatal("report must clearly label itself as a synthetic benchmark")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}
