package eval

import (
	"fmt"
	"sort"
	"strings"
)

// String renders a Report as a plain-text methodology + results document.
// The methodology header is not optional — any number pulled from this
// report into external claims (a README, a resume line) must carry this
// label with it: it is a synthetic, bootstrapped benchmark, not real
// pipeline output.
func (r Report) String() string {
	var b strings.Builder

	fmt.Fprintln(&b, "FlowCast classifier evaluation — SYNTHETIC benchmark")
	fmt.Fprintln(&b, "=====================================================")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "This is NOT real pipeline output. Trials are bootstrap-resampled")
	fmt.Fprintln(&b, "(with Gaussian jitter) from 4 real unmapped_tooshort_percent values")
	fmt.Fprintln(&b, "observed in FlowCast's canonical real run (results_real/, completed")
	fmt.Fprintln(&b, "2026-07-09; see internal/eval.EmpiricalNonOutlierValues). Injected")
	fmt.Fprintln(&b, "outliers are synthetic, at a controlled effect size in standard")
	fmt.Fprintln(&b, "deviations above that real empirical baseline.")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Trials: %d, seed: %d\n", r.Trials, r.Seed)
	fmt.Fprintf(&b, "Fixed threshold (frozen, mean+2sd of real baseline): %.3f%%\n", FixedThresholdPct)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Known limitation: at small sample sizes (n~5, matching FlowCast's real")
	fmt.Fprintln(&b, "run), the modified z-score's MAD estimate is noisy, giving the adaptive")
	fmt.Fprintln(&b, "classifier a real false-positive rate of roughly 20% on clean runs with")
	fmt.Fprintln(&b, "no true outlier — see the \"no outlier (clean run)\" row below.")
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "Aggregate results")
	fmt.Fprintln(&b, "------------------")
	fmt.Fprintf(&b, "%-32s %10s %10s %10s\n", "classifier", "precision", "recall", "F1")
	fmt.Fprintf(&b, "%-32s %10.3f %10.3f %10.3f\n", "adaptive (modified z-score)", r.Adaptive.Precision(), r.Adaptive.Recall(), r.Adaptive.F1())
	fmt.Fprintf(&b, "%-32s %10.3f %10.3f %10.3f\n", "fixed threshold", r.Fixed.Precision(), r.Fixed.Recall(), r.Fixed.F1())
	fmt.Fprintln(&b)

	if r.Adaptive.F1() > 0 || r.Fixed.F1() > 0 {
		delta := r.Adaptive.F1() - r.Fixed.F1()
		pct := 0.0
		if r.Fixed.F1() > 0 {
			pct = delta / r.Fixed.F1() * 100
		}
		fmt.Fprintf(&b, "Adaptive F1 - fixed F1 = %+.3f", delta)
		if r.Fixed.F1() > 0 {
			fmt.Fprintf(&b, " (%+.1f%% relative to fixed threshold)", pct)
		}
		fmt.Fprintln(&b)
		fmt.Fprintln(&b)
	}

	fmt.Fprintln(&b, "Breakdown by effect size bucket")
	fmt.Fprintln(&b, "--------------------------------")
	buckets := make([]string, 0, len(r.AdaptiveByBucket))
	for k := range r.AdaptiveByBucket {
		buckets = append(buckets, k)
	}
	sort.Strings(buckets)
	fmt.Fprintf(&b, "%-24s %-14s %10s %10s %10s\n", "bucket", "classifier", "precision", "recall", "F1")
	for _, bucket := range buckets {
		a := r.AdaptiveByBucket[bucket]
		f := r.FixedByBucket[bucket]
		fmt.Fprintf(&b, "%-24s %-14s %10.3f %10.3f %10.3f\n", bucket, "adaptive", a.Precision(), a.Recall(), a.F1())
		fmt.Fprintf(&b, "%-24s %-14s %10.3f %10.3f %10.3f\n", "", "fixed", f.Precision(), f.Recall(), f.F1())
	}

	return b.String()
}
