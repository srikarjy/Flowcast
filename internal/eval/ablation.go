package eval

import (
	"context"
	"strings"

	"flowcast/internal/classify"
	"flowcast/internal/narrator"
)

// causalPhrases are the heuristic markers ScoreUnsupported looks for. This
// is a simple keyword heuristic, not a semantic judge — it is intended to
// be a conservative, auditable proxy, not a precise one; see
// ScoreUnsupported's doc comment.
var causalPhrases = []string{
	"because", "due to", "caused by", "likely reflects", "is responsible for",
	"suggests that", "indicates that", "results from", "driven by", "explained by",
}

// ScoreUnsupported counts how many free-text claims make a causal
// assertion (per causalPhrases) without citing any evidence string present
// in evidence (field names/values from the finding, or terms from the
// reasoning document). This is a keyword heuristic, not a semantic judge:
// it will under- and over-count in individual cases, but it is applied
// identically to both the strict and naive narrator outputs, so the
// *relative* unsupported rate between them is the meaningful number, not
// either absolute count in isolation.
func ScoreUnsupported(claims []string, evidence []string) (unsupported, total int) {
	total = len(claims)
	for _, claim := range claims {
		lower := strings.ToLower(claim)
		isCausal := false
		for _, phrase := range causalPhrases {
			if strings.Contains(lower, phrase) {
				isCausal = true
				break
			}
		}
		if !isCausal {
			continue
		}
		cited := false
		for _, e := range evidence {
			if e == "" {
				continue
			}
			if strings.Contains(lower, strings.ToLower(e)) {
				cited = true
				break
			}
		}
		if !cited {
			unsupported++
		}
	}
	return unsupported, total
}

// AblationResult is the outcome of running both the strict (Narrate) and
// naive (NarrateNaive) narrators against the same findings, scored by
// ScoreUnsupported.
type AblationResult struct {
	StrictClaims      []string
	StrictUnsupported int
	StrictTotal       int
	NaiveClaims       []string
	NaiveUnsupported  int
	NaiveTotal        int
}

// RunNarratorAblation calls the real OpenAI API twice (via
// internal/narrator) against the same findings and reasoning doc: once
// through the strict, schema-constrained narrator, once through the naive
// control prompt. Costs a small amount against OPENAI_API_KEY — this is a
// live-API call, not a synthetic benchmark like Evaluate.
func RunNarratorAblation(ctx context.Context, findings []classify.Finding, reasoningDoc string) (AblationResult, error) {
	evidence := evidenceStrings(findings, reasoningDoc)

	strictClaims, err := narrator.Narrate(ctx, findings, reasoningDoc)
	if err != nil {
		return AblationResult{}, err
	}
	var strictText []string
	for _, c := range strictClaims {
		strictText = append(strictText, c.Claim+" "+c.EvidenceSource)
	}
	strictUnsupported, strictTotal := ScoreUnsupported(strictText, evidence)

	naiveClaims, err := narrator.NarrateNaive(ctx, findings, reasoningDoc)
	if err != nil {
		return AblationResult{}, err
	}
	naiveUnsupported, naiveTotal := ScoreUnsupported(naiveClaims, evidence)

	return AblationResult{
		StrictClaims:      strictText,
		StrictUnsupported: strictUnsupported,
		StrictTotal:       strictTotal,
		NaiveClaims:       naiveClaims,
		NaiveUnsupported:  naiveUnsupported,
		NaiveTotal:        naiveTotal,
	}, nil
}

// evidenceStrings extracts the citable field values from findings (e.g.
// "unmapped_tooshort_percent=5.75", sample names) that ScoreUnsupported
// checks naive claims against.
func evidenceStrings(findings []classify.Finding, reasoningDoc string) []string {
	var evidence []string
	for _, f := range findings {
		evidence = append(evidence, f.Sample, f.Rule)
		for _, part := range strings.Split(f.Detail, ",") {
			evidence = append(evidence, strings.TrimSpace(part))
		}
	}
	for _, line := range strings.Split(reasoningDoc, "\n") {
		line = strings.TrimSpace(line)
		if len(line) > 8 {
			evidence = append(evidence, line)
		}
	}
	return evidence
}
