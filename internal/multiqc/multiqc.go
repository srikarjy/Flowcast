// Package multiqc parses the subset of MultiQC's multiqc_data.json that
// FlowCast's classifier rules are traced to (see REASONING.md).
package multiqc

import (
	"encoding/json"
	"fmt"
	"os"
)

// StarSample holds the STAR alignment fields referenced by
// REASONING.md rule candidate 1. Field names and JSON keys match
// report_saved_raw_data.multiqc_star exactly as MultiQC emits them.
type StarSample struct {
	TotalReads            float64 `json:"total_reads"`
	UniquelyMappedPercent float64 `json:"uniquely_mapped_percent"`
	MultimappedPercent    float64 `json:"multimapped_percent"`
	MismatchRate          float64 `json:"mismatch_rate"`
	UnmappedTooShortPct   float64 `json:"unmapped_tooshort_percent"`
	MappedPercent         float64 `json:"mapped_percent"`
}

type rawData struct {
	MultiqcStar map[string]StarSample `json:"multiqc_star"`
}

type document struct {
	ReportSavedRawData rawData `json:"report_saved_raw_data"`
}

// LoadStarStats reads a MultiQC multiqc_data.json file and returns the
// per-sample STAR alignment stats keyed by sample name.
func LoadStarStats(path string) (map[string]StarSample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("multiqc: open %s: %w", path, err)
	}
	defer f.Close()

	var doc document
	if err := json.NewDecoder(f).Decode(&doc); err != nil {
		return nil, fmt.Errorf("multiqc: decode %s: %w", path, err)
	}
	if len(doc.ReportSavedRawData.MultiqcStar) == 0 {
		return nil, fmt.Errorf("multiqc: %s has no report_saved_raw_data.multiqc_star entries", path)
	}
	return doc.ReportSavedRawData.MultiqcStar, nil
}
