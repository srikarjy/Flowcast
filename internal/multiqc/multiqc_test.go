package multiqc

import (
	"os"
	"path/filepath"
	"testing"
)

// realMultiqcPath is FlowCast's canonical real-data baseline (STATUS.md
// §2.4). results_real/ is gitignored, so this test skips rather than fails
// when it isn't present locally, e.g. in CI.
const realMultiqcPath = "../../results_real/multiqc/star_salmon/multiqc_report_data/multiqc_data.json"

func TestLoadStarStats_RealFixture(t *testing.T) {
	if _, err := os.Stat(realMultiqcPath); err != nil {
		t.Skipf("real fixture not present (gitignored): %v", err)
	}

	stats, err := LoadStarStats(realMultiqcPath)
	if err != nil {
		t.Fatalf("LoadStarStats: %v", err)
	}

	want := map[string]float64{
		"RAP1_IAA_30M_REP1":   2.84,
		"RAP1_UNINDUCED_REP1": 3.16,
		"RAP1_UNINDUCED_REP2": 3.19,
		"WT_REP1":             5.75,
		"WT_REP2":             2.75,
	}
	if len(stats) != len(want) {
		t.Fatalf("expected %d samples, got %d: %+v", len(want), len(stats), stats)
	}
	for sample, wantPct := range want {
		got, ok := stats[sample]
		if !ok {
			t.Errorf("sample %q missing from parsed stats", sample)
			continue
		}
		if got.UnmappedTooShortPct != wantPct {
			t.Errorf("sample %q: unmapped_tooshort_percent = %.2f, want %.2f", sample, got.UnmappedTooShortPct, wantPct)
		}
	}
}

func TestLoadStarStats_EmptySection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(path, []byte(`{"report_saved_raw_data":{"multiqc_star":{}}}`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if _, err := LoadStarStats(path); err == nil {
		t.Fatal("expected error for empty multiqc_star section, got nil")
	}
}

func TestLoadStarStats_MissingFile(t *testing.T) {
	if _, err := LoadStarStats(filepath.Join(t.TempDir(), "does_not_exist.json")); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
