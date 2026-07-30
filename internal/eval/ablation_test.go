package eval

import "testing"

func TestScoreUnsupported(t *testing.T) {
	evidence := []string{"unmapped_tooshort_percent=5.75", "modified z-score=5.46"}

	claims := []string{
		"WT_REP1 has unmapped_tooshort_percent=5.75, well above the other samples.",                      // not causal, fine either way
		"WT_REP1 is likely contaminated because unmapped_tooshort_percent=5.75 is high.",                 // causal, cited
		"WT_REP1 failed likely due to a degraded RNA extraction batch.",                                  // causal, NOT cited
		"The sample suggests that a library prep error is responsible for the low mapping rate observed.", // causal, NOT cited
	}

	unsupported, total := ScoreUnsupported(claims, evidence)
	if total != 4 {
		t.Fatalf("total = %d, want 4", total)
	}
	if unsupported != 2 {
		t.Fatalf("unsupported = %d, want 2 (claims 3 and 4)", unsupported)
	}
}

func TestScoreUnsupported_Empty(t *testing.T) {
	unsupported, total := ScoreUnsupported(nil, []string{"x"})
	if unsupported != 0 || total != 0 {
		t.Fatalf("expected 0/0 for empty claims, got %d/%d", unsupported, total)
	}
}
