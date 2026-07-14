package classify

import (
	"testing"

	"flowcast/internal/multiqc"
)

func TestUnmappedTooShortOutliers(t *testing.T) {
	tests := []struct {
		name       string
		samples    map[string]multiqc.StarSample
		wantSample string // "" means no finding expected
	}{
		{
			name: "real run shape - WT_REP1 flagged",
			samples: map[string]multiqc.StarSample{
				"RAP1_IAA_30M_REP1":   {UnmappedTooShortPct: 2.84, MismatchRate: 0.19, MultimappedPercent: 8.90},
				"RAP1_UNINDUCED_REP1": {UnmappedTooShortPct: 3.16, MismatchRate: 0.20, MultimappedPercent: 8.51},
				"RAP1_UNINDUCED_REP2": {UnmappedTooShortPct: 3.19, MismatchRate: 0.17, MultimappedPercent: 7.92},
				"WT_REP1":             {UnmappedTooShortPct: 5.75, MismatchRate: 0.20, MultimappedPercent: 9.71},
				"WT_REP2":             {UnmappedTooShortPct: 2.75, MismatchRate: 0.15, MultimappedPercent: 10.41},
			},
			wantSample: "WT_REP1",
		},
		{
			name: "normal spread - no outlier",
			samples: map[string]multiqc.StarSample{
				"A": {UnmappedTooShortPct: 3.00},
				"B": {UnmappedTooShortPct: 3.10},
				"C": {UnmappedTooShortPct: 3.20},
				"D": {UnmappedTooShortPct: 2.95},
				"E": {UnmappedTooShortPct: 3.05},
			},
			wantSample: "",
		},
		{
			name: "all identical - MAD is zero, no findings",
			samples: map[string]multiqc.StarSample{
				"A": {UnmappedTooShortPct: 3.00},
				"B": {UnmappedTooShortPct: 3.00},
				"C": {UnmappedTooShortPct: 3.00},
			},
			wantSample: "",
		},
		{
			name: "fewer than 3 samples - no baseline to evaluate",
			samples: map[string]multiqc.StarSample{
				"A": {UnmappedTooShortPct: 2.00},
				"B": {UnmappedTooShortPct: 9.00},
			},
			wantSample: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := UnmappedTooShortOutliers(tt.samples)

			if tt.wantSample == "" {
				if len(findings) != 0 {
					t.Fatalf("expected no findings, got %d: %+v", len(findings), findings)
				}
				return
			}

			if len(findings) != 1 {
				t.Fatalf("expected exactly 1 finding, got %d: %+v", len(findings), findings)
			}
			if findings[0].Sample != tt.wantSample {
				t.Fatalf("expected flagged sample %q, got %q", tt.wantSample, findings[0].Sample)
			}
			if findings[0].Rule != "unmapped_tooshort_outlier" {
				t.Fatalf("expected rule 'unmapped_tooshort_outlier', got %q", findings[0].Rule)
			}
		})
	}
}
