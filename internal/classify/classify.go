// Package classify implements FlowCast's rule-based failure classifier.
// Every rule here must trace to a line in REASONING.md (Cardinal Rule 4).
package classify

import (
	"fmt"
	"sort"

	"flowcast/internal/multiqc"
)

// Finding is one classifier hit for one sample.
type Finding struct {
	Sample string
	Rule   string
	Detail string
}

// modifiedZScoreThreshold is the conventional flag threshold for the
// Iglewicz & Hoya (1993) modified z-score, not a value fit to any
// particular dataset.
const modifiedZScoreThreshold = 3.5

// UnmappedTooShortOutliers implements REASONING.md rule candidate 1:
// a within-run outlier check on STAR's unmapped_tooshort_percent using
// the modified z-score (median + MAD), flagging |z| > 3.5.
func UnmappedTooShortOutliers(samples map[string]multiqc.StarSample) []Finding {
	if len(samples) < 3 {
		return nil // too few samples in this run to compute a meaningful baseline
	}

	names := make([]string, 0, len(samples))
	values := make([]float64, 0, len(samples))
	for name, s := range samples {
		names = append(names, name)
		values = append(values, s.UnmappedTooShortPct)
	}

	med := median(values)
	deviations := make([]float64, len(values))
	for i, v := range values {
		deviations[i] = abs(v - med)
	}
	mad := median(deviations)
	if mad == 0 {
		return nil // all samples identical on this field; no spread to evaluate
	}

	var findings []Finding
	for i, name := range names {
		z := 0.6745 * abs(values[i]-med) / mad
		if z > modifiedZScoreThreshold {
			s := samples[name]
			findings = append(findings, Finding{
				Sample: name,
				Rule:   "unmapped_tooshort_outlier",
				Detail: fmt.Sprintf(
					"unmapped_tooshort_percent=%.2f, modified z-score=%.2f (median=%.2f, MAD=%.2f, threshold=%.1f); mismatch_rate=%.2f and multimapped_percent=%.2f are the ruled-out alternative causes per REASONING.md",
					values[i], z, med, mad, modifiedZScoreThreshold, s.MismatchRate, s.MultimappedPercent),
			})
		}
	}
	return findings
}

func median(vs []float64) float64 {
	sorted := append([]float64(nil), vs...)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
