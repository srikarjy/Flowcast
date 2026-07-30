# FlowCast — Causal Reasoning Document v1

Every classifier rule in FlowCast must trace to a line in this document (Cardinal Rule 4). This document only contains mechanisms grounded in real data from `results_real/multiqc/star_salmon/multiqc_report_data/multiqc_data.json` (real nf-core/rnaseq run, 2026-07-09, GEO GSE110004 yeast RAP1 data — see `PROGRESS.md` §5–6). No thresholds here are invented; where the real sample size is too small to defend an absolute cutoff, that's stated explicitly rather than papered over.

## Rule candidate 1: elevated STAR `unmapped_tooshort_percent`

**Real observed data (5 samples, this run):**

| Sample | uniquely_mapped% | unmapped_tooshort% | multimapped% | mismatch_rate |
|---|---|---|---|---|
| RAP1_IAA_30M_REP1 | 86.49 | 2.84 | 8.90 | 0.19 |
| RAP1_UNINDUCED_REP1 | 86.78 | 3.16 | 8.51 | 0.20 |
| RAP1_UNINDUCED_REP2 | 87.44 | 3.19 | 7.92 | 0.17 |
| WT_REP1 | 82.22 | **5.75** | 9.71 | 0.20 |
| WT_REP2 | 84.94 | 2.75 | 10.41 | 0.15 |

**Mechanism (Reported — documented STAR behavior, not something FlowCast measured itself):** STAR's default `outFilterScoreMinOverLread` / `outFilterMatchNminOverLread` = 0.66 requires 66% of a read's length to be matched/scored for it to count as mapped. Reads that align but fall short of that fraction are bucketed as `unmapped: too short`, distinct from `unmapped: too many mismatches` (mismatch_rate is separately reported and near-identical across all 5 samples here) and distinct from multimapping (also not elevated for WT_REP1). This isolates the mechanism: WT_REP1's lower `uniquely_mapped_percent` is driven almost entirely by the too-short bucket, not by sequencing error rate or multi-locus ambiguity.

**Ruled out as causes for WT_REP1's outlier (Observed, this run):**
- Not adapter/quality trimming — cutadapt `percent_trimmed` is uniform across all 5 samples (2.15–2.85%), WT_REP1 unremarkable.
- Not our own partial-download truncation artifact (see `PROGRESS.md` §4) — WT_REP1's two source FASTQ pairs (SRR6357070/71) are both full ~189MB slices, same as the other non-outlier samples; the one file that *was* visibly more truncated by our download method (SRR6357075, 125MB vs ~189MB) feeds RAP1_UNINDUCED_REP2, which is *not* the outlier.
- Not elevated mismatch rate or multimapping — both in line with the other 4 samples.

**Root cause of WT_REP1 specifically: Unknown.** Real candidates not yet distinguished — biological (this is a real wild-type vs. RAP1-depletion time-course; WT could carry different rRNA/contaminant load or RNA integrity) vs. batch/library-prep effect specific to this SRA run. Per Cardinal Rule 5, this stays Unknown until measured — FlowCast should not narrate a specific cause here.

**Threshold:** modified z-score (Iglewicz & Hoya 1993) on `unmapped_tooshort_percent` across the run's samples: `0.6745 × |x − median| / MAD`, flag if `> 3.5`. This is an established robust-statistics convention, not a value fit to this dataset (Cardinal Rule 4 — no inventing thresholds). On this run's real 5 samples, WT_REP1 scores 5.46 (flagged); all others score ≤ 0.86. Requires MAD > 0 (i.e., not all samples identical) to evaluate — with n=5 there's no claim beyond this one run's outlier check.

## Rule candidate 2: uniform FastQC `per_base_sequence_content` / `sequence_duplication_levels` fail — not yet a rule

**Real observed data:** all 5 samples, both mates, fail identically on exactly these two FastQC modules (`per_base_sequence_content: fail`, `sequence_duplication_levels: fail`); all other modules pass uniformly too. This is the same pattern seen in the earlier toy-data run (`PROGRESS.md` §3).

**Status: open, not a rule.** Two live hypotheses, undistinguished by current data:
1. **Reported (literature-known):** standard polyA-selected mRNA-seq libraries characteristically fail these two specific modules — random-hexamer priming bias skews per-base composition, and highly-expressed transcripts cause disproportionate duplication — independent of sample condition. If true, this is expected background, not a failure signal, and should never fire as a FlowCast rule.
2. **Unknown/unruled-out:** a dataset- or extraction-method-level confound (this is the second time in a row it's been perfectly uniform across biologically distinct conditions).

Not resolving this from the current single run. Needs either literature grounding (checking published FastQC guidance for standard RNA-seq) or a second independent real dataset before it can become — or be explicitly excluded as — a rule.

## Next
Rule candidate 1 (relative `unmapped_tooshort_percent` outlier) is the first rule with enough traceable grounding to implement in the Go classifier. Rule candidate 2 stays out of the classifier until resolved.
