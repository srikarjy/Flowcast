// Package eval measures FlowCast's classifier and narrator against
// reproducible, honestly-labeled benchmarks. Nothing in this package is
// real pipeline output — synthetic trials here are bootstrapped from real
// values (see EmpiricalNonOutlierValues) but are explicitly synthetic, and
// every report says so. This exists because a single real run with 5
// samples (1 outlier) isn't enough data to compute a meaningful F1 score on
// its own.
package eval

import (
	"fmt"
	"math"
	"math/rand"

	"flowcast/internal/classify"
	"flowcast/internal/multiqc"
)

// EmpiricalNonOutlierValues are the four real unmapped_tooshort_percent
// values from FlowCast's canonical real run (excluding WT_REP1, the one
// real outlier): results_real/multiqc/star_salmon/multiqc_report_data/
// multiqc_data.json, run completed 2026-07-09 (STATUS.md §2.4). This is
// the resampling pool synthetic trials are bootstrapped from.
var EmpiricalNonOutlierValues = []float64{2.84, 3.16, 3.19, 2.75}

// nonOutlierMean and nonOutlierStd are computed once from
// EmpiricalNonOutlierValues at package init.
var nonOutlierMean, nonOutlierStd = meanStd(EmpiricalNonOutlierValues)

// FixedThresholdPct is a static QC cutoff representing a conventional
// fixed-threshold rule: the real non-outlier mean plus 2 standard
// deviations, computed once from EmpiricalNonOutlierValues and frozen —
// unlike the classifier's per-run adaptive median/MAD approach, this value
// does not change no matter what a given synthetic run's own spread looks
// like. That gap (adaptive vs. frozen) is exactly what this benchmark is
// evaluating.
var FixedThresholdPct = nonOutlierMean + 2*nonOutlierStd

// Trial is one synthetic run: a set of sample values and the ground truth
// of whether (and which) sample is a true injected outlier.
type Trial struct {
	Values      map[string]float64
	TrueOutlier string // "" if no outlier was injected
	EffectSize  float64
}

// GenerateTrial draws sampleSize non-outlier values from a Normal
// distribution fitted to EmpiricalNonOutlierValues (a parametric bootstrap:
// mean and std estimated from the 4 real values, not resampled discretely
// from them — resampling with replacement from only 4 discrete points
// collided too often at small sample sizes, artificially collapsing MAD).
// Optionally injects one synthetic outlier at effectSize standard
// deviations above the empirical non-outlier mean.
func GenerateTrial(rng *rand.Rand, sampleSize int, injectOutlier bool, effectSize float64) Trial {
	values := make(map[string]float64, sampleSize)
	outlierIdx := -1
	if injectOutlier {
		outlierIdx = rng.Intn(sampleSize)
	}

	for i := 0; i < sampleSize; i++ {
		name := fmt.Sprintf("SYN_%d", i)
		if i == outlierIdx {
			values[name] = nonOutlierMean + effectSize*nonOutlierStd
			continue
		}
		values[name] = nonOutlierMean + rng.NormFloat64()*nonOutlierStd
	}

	t := Trial{Values: values, EffectSize: effectSize}
	if outlierIdx >= 0 {
		t.TrueOutlier = fmt.Sprintf("SYN_%d", outlierIdx)
	}
	return t
}

// AdaptiveFlags runs FlowCast's real classifier (modified z-score,
// internal/classify) against a trial's values.
func AdaptiveFlags(t Trial) map[string]bool {
	samples := make(map[string]multiqc.StarSample, len(t.Values))
	for name, v := range t.Values {
		samples[name] = multiqc.StarSample{UnmappedTooShortPct: v}
	}
	findings := classify.UnmappedTooShortOutliers(samples)
	flagged := make(map[string]bool, len(findings))
	for _, f := range findings {
		flagged[f.Sample] = true
	}
	return flagged
}

// FixedThresholdFlags flags every sample strictly above FixedThresholdPct —
// the baseline this benchmark compares the adaptive classifier against.
func FixedThresholdFlags(t Trial) map[string]bool {
	flagged := make(map[string]bool)
	for name, v := range t.Values {
		if v > FixedThresholdPct {
			flagged[name] = true
		}
	}
	return flagged
}

// Counts accumulates true/false positives and false negatives across
// trials for one classifier, treating "correctly flags the true outlier
// and nothing else" as the target behavior.
type Counts struct {
	TP, FP, FN int
}

func (c *Counts) Add(t Trial, flagged map[string]bool) {
	if t.TrueOutlier == "" {
		c.FP += len(flagged) // any flag on a clean run is a false positive
		return
	}
	if flagged[t.TrueOutlier] {
		c.TP++
	} else {
		c.FN++
	}
	for name := range flagged {
		if name != t.TrueOutlier {
			c.FP++
		}
	}
}

func (c Counts) Precision() float64 {
	if c.TP+c.FP == 0 {
		return 0
	}
	return float64(c.TP) / float64(c.TP+c.FP)
}

func (c Counts) Recall() float64 {
	if c.TP+c.FN == 0 {
		return 0
	}
	return float64(c.TP) / float64(c.TP+c.FN)
}

func (c Counts) F1() float64 {
	p, r := c.Precision(), c.Recall()
	if p+r == 0 {
		return 0
	}
	return 2 * p * r / (p + r)
}

// Report is the result of Evaluate: aggregate counts for both classifiers,
// plus a coarse breakdown by effect size so the report can show where the
// fixed threshold starts missing outliers that the adaptive classifier
// still catches.
type Report struct {
	Trials           int
	Seed             int64
	Adaptive, Fixed  Counts
	AdaptiveByBucket map[string]*Counts
	FixedByBucket    map[string]*Counts
}

func bucketFor(effectSize float64, injected bool) string {
	switch {
	case !injected:
		return "no outlier (clean run)"
	case effectSize < 3:
		return "small effect (0-3σ)"
	case effectSize < 8:
		return "medium effect (3-8σ)"
	default:
		return "large effect (8σ+)"
	}
}

// Evaluate runs `trials` seeded synthetic trials through both the adaptive
// classifier and the fixed-threshold baseline, and aggregates precision /
// recall / F1 for each. Half the trials inject no outlier (testing false
// positive behavior on a clean run); the other half inject one outlier at
// an effect size drawn uniformly from [0, 12] standard deviations, sample
// size drawn uniformly from [4, 8].
func Evaluate(trials int, seed int64) Report {
	rng := rand.New(rand.NewSource(seed))
	report := Report{
		Trials:           trials,
		Seed:             seed,
		AdaptiveByBucket: map[string]*Counts{},
		FixedByBucket:    map[string]*Counts{},
	}

	for i := 0; i < trials; i++ {
		injectOutlier := i%2 == 1
		sampleSize := 4 + rng.Intn(5) // [4,8]
		effectSize := rng.Float64() * 12

		t := GenerateTrial(rng, sampleSize, injectOutlier, effectSize)

		adaptiveFlags := AdaptiveFlags(t)
		fixedFlags := FixedThresholdFlags(t)

		report.Adaptive.Add(t, adaptiveFlags)
		report.Fixed.Add(t, fixedFlags)

		bucket := bucketFor(effectSize, injectOutlier)
		if report.AdaptiveByBucket[bucket] == nil {
			report.AdaptiveByBucket[bucket] = &Counts{}
		}
		if report.FixedByBucket[bucket] == nil {
			report.FixedByBucket[bucket] = &Counts{}
		}
		report.AdaptiveByBucket[bucket].Add(t, adaptiveFlags)
		report.FixedByBucket[bucket].Add(t, fixedFlags)
	}

	return report
}

func meanStd(vs []float64) (mean, std float64) {
	sum := 0.0
	for _, v := range vs {
		sum += v
	}
	mean = sum / float64(len(vs))

	sqDiff := 0.0
	for _, v := range vs {
		d := v - mean
		sqDiff += d * d
	}
	std = math.Sqrt(sqDiff / float64(len(vs)))
	return mean, std
}
