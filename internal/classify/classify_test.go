package classify

import (
	"testing"

	"flowcast/internal/multiqc"
	"flowcast/internal/nftrace"
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

func TestFailedTasks(t *testing.T) {
	tasks := []nftrace.Task{
		{TaskID: "30", Name: "NFCORE_RNASEQ:RNASEQ:ALIGN_STAR:STAR_ALIGN (WT_REP2)", Status: "FAILED", Exit: "137", Duration: "4m 33s"},
		{TaskID: "31", Name: "NFCORE_RNASEQ:RNASEQ:ALIGN_STAR:STAR_ALIGN (RAP1_UNINDUCED_REP1)", Status: "ABORTED", Exit: "-"},
		{TaskID: "1", Name: "NFCORE_RNASEQ:RNASEQ:FASTQC (WT_REP1)", Status: "COMPLETED", Exit: "0"},
	}

	findings := FailedTasks(tasks)
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Sample != "WT_REP2" {
		t.Fatalf("expected flagged sample %q, got %q", "WT_REP2", findings[0].Sample)
	}
	if findings[0].Rule != "task_failed" {
		t.Fatalf("expected rule 'task_failed', got %q", findings[0].Rule)
	}
}

// realFailedTracePath is a real Nextflow execution_trace.txt from a
// deliberately memory-starved nf-core/rnaseq run (STAR_ALIGN capped at
// 400MB): WT_REP2's STAR_ALIGN task was genuinely SIGKILL'd (exit 137)
// after real alignment work, not a fabricated fixture.
const realFailedTracePath = "testdata/execution_trace_star_align_oom.txt"

func TestFailedTasks_RealFixture(t *testing.T) {
	tasks, err := nftrace.LoadTasks(realFailedTracePath)
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}

	findings := FailedTasks(tasks)
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Sample != "WT_REP2" {
		t.Fatalf("expected flagged sample %q, got %q", "WT_REP2", findings[0].Sample)
	}
}
