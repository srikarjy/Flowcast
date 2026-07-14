# FlowCast — Progress Log

This tracks what's actually been done, in line with the Cardinal Rules in `CLAUDE.md` — nothing here is claimed unless it happened on real data.

## Where we are

The Go parser, classifier, and narrator are all written, wired together in one CLI (`cmd/flowcast`), and **verified end-to-end against real data, including a real narrator run** (§8, 2026-07-13). Cardinal Rule 6's gate (one real end-to-end run before any supporting infrastructure) is now satisfied. This is v1's locked scope, complete.

v2 (SQLite event log, `replay` command, Python SDK + narrator) is written and **verified against the real WT_REP1 run** (§9, 2026-07-14), including the required real Go-only replay proof and real interleaved Go+Python replay proof. The narrator (both languages) now calls OpenAI, not Claude, per the v3 scope amendment in `CLAUDE.md` — the v1 Claude run in §8 remains an accurate historical record and is not rewritten. Remaining v2 item: README section on ordering tradeoffs.

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

### 7. Go parser + classifier — implemented, verified on real data
Built `cmd/flowcast` (CLI entry point), `internal/nftrace` (tab-separated `execution_trace.txt` parser), `internal/multiqc` (JSON parser for the `report_saved_raw_data.multiqc_star` section of `multiqc_data.json`), and `internal/classify` (rule-based classifier).

Implemented **Rule 1 only** (`UnmappedTooShortOutliers`): the modified z-score outlier check from `REASONING.md` rule candidate 1. Rule candidate 2 (uniform FastQC fail flags) is correctly *not* implemented — it's still an open question in `REASONING.md`, not a traceable rule (Cardinal Rule 4).

**Verified real run** (2026-07-13), not just built:
```
go run ./cmd/flowcast -trace results_real/pipeline_info/execution_trace_2026-07-09_23-42-27.txt \
  -multiqc results_real/multiqc/star_salmon/multiqc_report_data/multiqc_data.json
```
→ parses 208 real trace tasks (all `COMPLETED`/`CACHED`), parses 5 real STAR samples, classifier flags `WT_REP1` with modified z-score 5.46 — matching the numbers documented in `REASONING.md` exactly.

### 8. Go narrator — implemented and verified on a real run (2026-07-13)
Built `internal/narrator`, wired into `cmd/flowcast` after the classifier stage. Sends classifier findings plus the full text of `REASONING.md` to Claude (`claude-opus-4-8`, structured JSON output via `output_config.format`), with a system prompt enforcing Cardinal Rule 5: every claim must carry a `confidence_tag` of `Observed`, `Reported`, or `Unknown`.

Ran the real CLI end-to-end with `ANTHROPIC_API_KEY` configured, against the same real trace/multiqc files from §7:
```
go run ./cmd/flowcast \
  -multiqc results_real/multiqc/star_salmon/multiqc_report_data/multiqc_data.json \
  -trace results_real/pipeline_info/execution_trace_2026-07-09_23-42-27.txt \
  -reasoning REASONING.md
```
**This is the first real narrator output** — 13 claims returned, each carrying a `confidence_tag`:
- 7 tagged `Observed` (classifier numbers, ruled-out alternative causes, read directly from findings/REASONING.md).
- 5 tagged `Reported` (statistical convention citation, STAR mechanism explanation, trimming/truncation rule-outs, the n=5 sample-size caveat — all attributed to a REASONING.md section, not asserted as fact).
- **1 tagged `Unknown`**: "The specific root cause of WT_REP1's outlier status is unresolved and remains Unknown; the classifier does not attribute it to a biological, technical, or batch-effect cause." — the narrator correctly refused to guess at causation `REASONING.md` explicitly leaves open, per Cardinal Rule 5.

This satisfies Cardinal Rule 6: one real end-to-end diagnosis now exists on real data, narrator included. **v1's locked scope (Go CLI, stdlib parsing, rule-based classifier, Claude API narrator with confidence-tagged claims) is complete and verified.**

### 9. v2 event log + replay — implemented and verified on the real WT_REP1 run (2026-07-14)
Built `internal/eventlog` (SQLite via `modernc.org/sqlite`, pure Go, no CGO), wired as an optional `-eventlog <path>` flag into `cmd/flowcast` emitting `trace_parsed`, `classify.finding`, and `narrator.claim` events. Added `flowcast replay -eventlog <path>`, which lists every event ordered by `ts ASC, id ASC` regardless of which language wrote it. Added a Python SDK (`python/flowcast_sdk/eventlog.py`, same schema/pragmas) and `python/narrate.py`, an independently-implemented second narrator that reads back the Go classifier's `finding` events from the shared log and appends its own `narrator.claim` events, `source_lang=python`.

**Note on Rule 3's v3 amendment (OpenAI, not Claude):** this run used `OPENAI_API_KEY`, per the v3 scope amendment in `CLAUDE.md` — a credential-availability decision made before this run, not something decided here.

Ran the real proof sequence against the same real trace/multiqc files from §7, in order, per the v2 locked scope:

1. **Go-only, end-to-end, with `-eventlog` on:**
   ```
   go run ./cmd/flowcast -trace results_real/pipeline_info/execution_trace_2026-07-09_23-42-27.txt \
     -multiqc results_real/multiqc/star_salmon/multiqc_report_data/multiqc_data.json \
     -reasoning REASONING.md -eventlog flowcast_events.db
   ```
   Real OpenAI (`gpt-4o`) narrator run produced 4 claims: 2 `Observed`, 1 `Reported`, 1 `Unknown` ("The root cause of the elevated unmapped_tooshort_percent in WT_REP1 remains unresolved..." — matching `REASONING.md`'s own open Unknown, per Cardinal Rule 5). Fewer claims than the v1 Claude run (§8, 13 claims) — same underlying finding, different model, not a regression in the confidence-tagging behavior itself.

2. **Go-only replay proof:** `go run ./cmd/flowcast replay -eventlog flowcast_events.db` — printed back all 6 events (1 `trace_parsed`, 1 `finding`, 4 `narrator claim`), all `source_lang=go`, correctly ordered by timestamp.

3. **Python narrator, reading the Go-written log:**
   ```
   cd python && python3 narrate.py --eventlog ../flowcast_events.db --reasoning ../REASONING.md
   ```
   Read back the real `WT_REP1` finding Go wrote in step 1, independently called OpenAI, and produced its own 4 claims (2 `Observed`, 1 `Reported`, 1 `Unknown`) — substantively the same conclusions as the Go narrator, phrased differently, confirming both narrators are enforcing the same confidence-tagging rule against the same real evidence.

4. **Interleaved replay proof:** `go run ./cmd/flowcast replay -eventlog flowcast_events.db` again — now shows all 10 events (6 `go`, 4 `python`) in one list, correctly ordered by timestamp across languages.

**Honest limitation, stated plainly (not glossed over):** steps 1 and 3 ran sequentially — Go's last event was at `02:19:39.1275Z`, Python's first event was at `02:20:14.854Z`, about 35 seconds later. This proves the shared log and timestamp-ordered replay correctly interleave events *written by different languages*, but it is **not** a proof of correct behavior under simultaneous concurrent writes from both languages at once — `internal/eventlog/eventlog.go`'s doc comment already flags this as untested. That remains an open limitation, not resolved by this run.

This satisfies the v2 locked scope's real-data requirement (Cardinal Rule 2): the event log and replay are demonstrated against the real `WT_REP1` run, not asserted as working.

## Next steps
v1 and the v2 event-log/replay proof are both done and verified on real data. Remaining v2 locked-scope item: a README section on the ordering tradeoffs observed in §9 (in progress). Per Cardinal Rule 1, further scope beyond that needs a real bug/limitation hit — the concurrent-write limitation noted in §9 is a candidate if it's ever actually hit, not a reason to build for it now. Other candidate next discussions (not yet decided): a second classifier rule (only if traceable to a resolved `REASONING.md` question — rule candidate 2 is still open), basic CI (workflow already added, not yet observed running on a push). See `PORTFOLIO_REVIEW (1).md` for one outside perspective — still needs reconciling against the Cardinal Rules before acting on any of it.

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
