# FlowCast — Progress Log

This tracks what's actually been done, in line with the Cardinal Rules in `CLAUDE.md` — nothing here is claimed unless it happened on real data.

## Where we are

Still in the "get real data" phase (Cardinal Rule 6: one real end-to-end run before any supporting infrastructure). No FlowCast code (Go parser, classifier, narrator) has been written yet — everything so far has been getting genuine, real nf-core/rnaseq pipeline output to build against.

## What's been done

### 1. Environment setup
- Installed Miniconda, created an isolated `flowcast-nf` conda environment containing Nextflow (so nothing pollutes system Java/PATH).
- Installed and started Docker Desktop (used as the container engine for pipeline execution).

### 2. First real pipeline run (baseline, toy test data)
Ran `nf-core/rnaseq` (release `3.26.0`) against its own tiny built-in test dataset, on an 8GB RAM MacBook Air.

Real problems hit and resolved along the way (all genuine tooling issues, not FlowCast design issues):
- Nextflow version too new for an older pipeline release's config syntax → resolved by using the latest pipeline release instead of an old pinned one.
- `nf-validation` plugin bug incorrectly reporting real, reachable remote test-data URLs as missing → resolved by upgrading to the pipeline release that replaced it.
- Local pipeline revision cache got stuck on an old revision → resolved by wiping `~/.nextflow/assets/nf-core` and re-pulling.
- One process requested 12GB memory, machine only has 8GB → resolved with a custom `low_mem.config` forcing a hard `resourceLimits` cap (4 CPU / 6GB / 4h).
- A Docker image download was interrupted mid-pull → resolved by retrying.

**Result:** pipeline completed successfully — 221 processes succeeded, 13 cached, real `trace.txt` and `multiqc_data.json` produced.

### 3. Honesty check on the baseline run
Inspected the real `multiqc_data.json` and found every single sample (8/8) failed FastQC's `per_base_sequence_content` and `sequence_duplication_levels` checks identically, regardless of biological condition. STAR alignment rates were all healthy (89–98%).

**Flagged as a problem, not a finding:** uniform failure across every sample regardless of condition looks like an artifact of the tiny (~50K read) toy test dataset, not real biological signal. Building a classifier rule on this would violate Cardinal Rule 4 (rules must trace to real causal reasoning, not test-fixture noise). Decision (with user): get more realistic data instead of building on this.

### 4. Sourcing real, larger data
- Identified that the toy test data is a subsample of a real published study: **GEO GSE110004** (yeast RAP1 transcription factor depletion), SRA runs `SRR6357070`–`SRR6357076`.
- Checked full run sizes via ENA — full runs are 47–68M reads each, ~40GB total combined. Too large for practical use here.
- Downloaded **real, partial slices** instead: first ~180MB (compressed) of each mate via HTTP range requests directly from ENA, decompressed (tolerating truncation), trimmed both mates to equal complete-record counts, recompressed. Result: **2.88M–3.74M real read pairs per sample** (vs. ~50K in the toy set) — genuinely real reads, just not full depth.
- Downloaded the **real, full S. cerevisiae reference genome + GTF** (Ensembl R64-1-1) to replace the toy test's truncated single-locus reference.
- Built `samplesheet.csv` pointing to these real local files, paired-end for all 7 samples (the toy test only used single-end reads for some samples; real full data has both mates for everyone).

### 5. Second pipeline run (real, larger data) — complete
Ran:
```bash
nextflow run nf-core/rnaseq -r 3.26.0 -profile docker \
  --input samplesheet.csv --fasta reference/genome.fasta.gz --gtf reference/genes.gtf.gz \
  --outdir results_real -c low_mem.config
```
Completed 2026-07-09. Real files: `results_real/pipeline_info/execution_trace_2026-07-09_23-42-27.txt` and `results_real/multiqc/star_salmon/multiqc_report_data/multiqc_data.json`. The 7 samplesheet rows merged into 5 samples per nf-core/rnaseq's replicate-lane merging (WT_REP1 and RAP1_UNINDUCED_REP2 each combine two SRA runs).

### 6. Inspected real `multiqc_data.json` for sample-to-sample QC signal
Checked whether this run has real differentiated signal or repeats the toy run's uniform-artifact problem (see §3).

**Real, differentiated numeric signal found** — not flat across samples:
- STAR `uniquely_mapped_percent`: 82.22% (WT_REP1) to 87.44% (RAP1_UNINDUCED_REP2), a genuine ~5-point spread.
- FastQC `percent_duplicates`: 58.17%–68.59%, samtools duplicate rates, error rates, and insert sizes all vary continuously by sample too.

**One repeat of the toy-run pattern, flagged not concluded:** FastQC's categorical pass/fail flags are still identical across all 5 samples — `per_base_sequence_content: fail` and `sequence_duplication_levels: fail`, every sample, both mates. This could be a genuine, expected mRNA-seq pattern (random-hexamer priming bias + highly-expressed-transcript duplication commonly fail these two specific modules in real polyA RNA-seq, independent of sample) or still a dataset-level confound — not yet determined. Per Cardinal Rule 4, this needs to be resolved in the causal reasoning doc before any rule is built on it, either way.

## Next steps
1. Write the one-page causal biology/technical reasoning document (Cardinal Rule 4), starting with the STAR `uniquely_mapped_percent` spread (clean real signal) and resolving whether the uniform FastQC fail flags are expected biology or a confound.
2. Derive the first classifier rule from that doc, traced to specific real observed fields.
3. Only then start the Go parser + classifier + Claude narrator (v1 locked scope).

## Real numbers so far (for reference)
| Sample | Real read pairs (partial slice) |
|---|---|
| SRR6357070 (WT_REP1) | 3,531,698 |
| SRR6357071 (WT_REP1) | 3,601,677 |
| SRR6357072 (WT_REP2) | 3,743,579 |
| SRR6357073 (RAP1_UNINDUCED_REP1) | 3,577,795 |
| SRR6357074 (RAP1_UNINDUCED_REP2) | 3,546,971 |
| SRR6357075 (RAP1_UNINDUCED_REP2) | 2,878,514 |
| SRR6357076 (RAP1_IAA_30M_REP1) | 3,545,583 |
