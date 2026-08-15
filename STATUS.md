# FlowCast — Status, Roadmap, and Architecture Decisions

Last updated: 2026-08-15. Written against commit `9509208` (branch `feat/add-opentelemetry-tracing`, PR #2 open against `main`).

This document consolidates what has actually been built and verified, what remains, and the architectural decisions taken along the way — including the ones that were deliberate exceptions to the project's own rules.

Everything in the "done" section below happened on real data from a real pipeline run. Nothing here is estimated or aspirational; the roadmap section is clearly separated for that reason.

---

## 1. Where the project stands

FlowCast is a Go CLI that diagnoses nf-core/rnaseq runs. It parses Nextflow's `execution_trace.txt` and MultiQC's `multiqc_data.json`, applies a rule-based classifier to flag outlier samples and outright task failures, and sends the findings to an LLM narrator that returns confidence-tagged claims. A shared SQLite event log lets an independently written Python narrator consume the same findings, and `flowcast replay` plays back both languages' events in one timestamp-ordered stream. OpenTelemetry spans instrument every pipeline stage.

**v1 scope: complete and verified end-to-end on real data.**
**v2 scope (event log, replay, Python SDK): complete and verified, with one documented limitation.**
**v3 scope (task-level failure classifier, rule candidate 3): complete and verified end-to-end, including the narrator (§2.9).**

Current build state: `go build ./...` and `go vet ./...` pass; `go test ./...` passes. Test coverage: `internal/nftrace`, `internal/multiqc`, `internal/classify`, `internal/eval` — `internal/narrator`, `internal/eventlog`, and `internal/tracing` still have none.

CI (`.github/workflows/ci.yml`) has been confirmed running green on real pushes and PR merges (verified via `gh run list`, 2026-08-15) — this was previously an open question (§3, old) and is now resolved.

---

## 2. What has been done

### 2.1 Environment and first pipeline run (baseline, toy data)

Miniconda with an isolated `flowcast-nf` conda env (Nextflow 26.04.4) plus Docker Desktop, on an 8GB MacBook Air. Ran `nf-core/rnaseq` 3.26.0 against its built-in test dataset.

Real tooling problems hit and resolved: a Nextflow/pipeline version mismatch, an `nf-validation` plugin bug misreporting reachable test-data URLs as missing, a stuck local revision cache (fixed by wiping `~/.nextflow/assets/nf-core`), a process requesting 12GB on an 8GB machine (fixed with `low_mem.config` capping resources at 4 CPU / 6GB / 4h), and an interrupted Docker image pull.

Result: 221 processes succeeded, 13 cached, real trace and MultiQC files produced.

### 2.2 Honesty check that changed the plan

Inspecting the baseline `multiqc_data.json` showed all 8 samples failing FastQC's `per_base_sequence_content` and `sequence_duplication_levels` identically, regardless of biological condition. That uniformity looked like an artifact of the ~50K-read toy dataset rather than biological signal, so it was flagged as a problem rather than treated as a finding. Building a classifier rule on it would have meant inventing a threshold from test-fixture noise. The decision was to get more realistic data first.

This is the single most consequential decision in the project's history: it cost a rebuild of the data pipeline and bought a classifier grounded in something real.

### 2.3 Sourcing real data

The toy test data turned out to be a subsample of a real published study — GEO GSE110004 (yeast RAP1 transcription factor depletion), SRA runs SRR6357070–SRR6357076. Full runs are 47–68M reads each (~40GB total), impractical here.

Instead: real partial slices, taking the first ~180MB compressed of each mate via HTTP range requests from ENA, decompressed tolerating truncation, both mates trimmed to equal complete-record counts, recompressed. That yielded 2.88M–3.74M real read pairs per sample — genuinely real reads, just not full depth. Paired with the real full S. cerevisiae reference genome and GTF (Ensembl R64-1-1).

| Sample (SRA run) | Real read pairs |
|---|---|
| SRR6357070 (WT_REP1) | 3,531,698 |
| SRR6357071 (WT_REP1) | 3,601,677 |
| SRR6357072 (WT_REP2) | 3,743,579 |
| SRR6357073 (RAP1_UNINDUCED_REP1) | 3,577,795 |
| SRR6357074 (RAP1_UNINDUCED_REP2) | 3,546,971 |
| SRR6357075 (RAP1_UNINDUCED_REP2) | 2,878,514 |
| SRR6357076 (RAP1_IAA_30M_REP1) | 3,545,583 |

### 2.4 Second pipeline run (real data) — the canonical baseline

Completed 2026-07-09 into `results_real/`. The 7 samplesheet rows merged into 5 samples via nf-core/rnaseq's replicate-lane merging. Canonical artifacts:

- `results_real/pipeline_info/execution_trace_2026-07-09_23-42-27.txt` (208 tasks)
- `results_real/multiqc/star_salmon/multiqc_report_data/multiqc_data.json`

This run produced genuinely differentiated signal — STAR `uniquely_mapped_percent` spanning 82.22% (WT_REP1) to 87.44% (RAP1_UNINDUCED_REP2), and continuously varying duplicate/error/insert-size metrics. The FastQC categorical pass/fail flags were still uniform across all 5 samples, which was flagged and left unresolved rather than concluded.

### 2.5 Go parser and classifier — verified on real data (2026-07-13)

- `internal/nftrace` — tab-separated `execution_trace.txt` parser, standard library `encoding/csv` with `Comma = '\t'`, validating the 14-column header exactly.
- `internal/multiqc` — decodes only the `report_saved_raw_data.multiqc_star` section, into a struct whose JSON tags match MultiQC's emitted keys exactly.
- `internal/classify` — Rule 1 only (`UnmappedTooShortOutliers`): modified z-score (Iglewicz & Hoya 1993) over `unmapped_tooshort_percent`, `0.6745 × |x − median| / MAD`, flagging `> 3.5`. Returns nil for fewer than 3 samples or when MAD is 0.

Verified run: parses 208 real tasks (all COMPLETED/CACHED), 5 real STAR samples, flags `WT_REP1` at modified z-score 5.46 — matching the reasoning document's numbers exactly. All other samples score ≤ 0.86.

Rule candidate 2 (uniform FastQC fails) was deliberately **not** implemented, because it remains an open question rather than a traceable mechanism.

### 2.6 Go narrator — first real end-to-end diagnosis (2026-07-13)

`internal/narrator` sends classifier findings plus the full reasoning document to an LLM with structured JSON output, under a system prompt requiring every claim to carry `confidence_tag` ∈ {Observed, Reported, Unknown} and an `evidence_source` citing a specific field or document section.

The first real narrator run (then on Claude `claude-opus-4-8`) returned 13 claims: 7 Observed, 5 Reported, and 1 Unknown — the narrator correctly refusing to guess at a root cause the reasoning document explicitly leaves open. That refusal is the behavior the whole project exists to demonstrate.

This satisfied the gate requiring one real end-to-end diagnosis before any supporting infrastructure, and completed v1.

### 2.7 v2: event log, replay, Python SDK — verified (2026-07-14)

- `internal/eventlog` — SQLite via `modernc.org/sqlite` (pure Go, no CGO), single `events` table, opened WAL with a 5s busy timeout. Optional `-eventlog <path>` flag emits `trace_parsed`, `classify.finding`, and `narrator.claim` events.
- `flowcast replay -eventlog <path>` — lists all events ordered by `ts ASC, id ASC`, regardless of writing language.
- `python/flowcast_sdk/eventlog.py` + `python/narrate.py` — a second, independently implemented narrator sharing the schema and pragmas, reading back Go's `finding` events and appending its own claims with `source_lang=python`.

The real proof sequence ran against the same real WT_REP1 data: a Go-only run producing 4 OpenAI (`gpt-4o`) claims (2 Observed, 1 Reported, 1 Unknown); a Go-only replay showing all 6 events correctly ordered; the Python narrator independently producing its own 4 claims (2 Observed, 1 Reported, 1 Unknown) — substantively the same conclusions, phrased differently, from the same evidence; and an interleaved replay showing all 10 events (6 Go, 4 Python) correctly ordered across languages.

The two narrators independently converging on the same confidence tags against the same evidence is the strongest evidence so far that the tagging discipline lives in the prompt and reasoning document, not in one model's idiosyncrasies.

### 2.8 CI and release workflows

`.github/workflows/ci.yml` (build + vet + test on push/PR to main, Go 1.25) and `.github/workflows/release.yml` (builds the binary and publishes a GitHub Release on `v*.*.*` tags). **Confirmed running green** on real pushes and PR merges (`gh run list`, 2026-08-15) — resolves the open question below in §3.1 (old).

### 2.9 v3: task-level failure classifier — verified against a real induced failure (2026-08-15)

Every prior run had *succeeded* — every rule up to this point was a within-run outlier check on healthy data, not a failure classifier, which was the project's most significant real gap (previously tracked as §3.4, old). Closing it meant deliberately inducing a real pipeline failure, not fabricating a trace.

Done by capping `STAR_ALIGN`'s memory at 400MB (`low_mem_fail.config`) — well below what STAR's alignment pass on the yeast genome actually needs — and rerunning the real pipeline against the same real data as the canonical baseline (§2.4). `STAR_ALIGN(WT_REP2)` completed genome loading and a real 1st-pass mapping (~4m26s of real work), started 2nd-pass mapping, and was genuinely SIGKILL'd: `execution_trace.txt` records `status=FAILED exit=137 duration=4m 33s`.

`classify.FailedTasks` (rule candidate 3, documented in `REASONING.md`) flags any trace task with `status == FAILED`. Unlike rule candidate 1, it needs no baseline or sample spread — a single failed task is itself the finding — so it fires independent of run size and independent of whether MultiQC output exists for the failed sample (a real failure often means QC output for that sample was never produced). `-multiqc` is now optional on `flowcast run` for this reason.

Real trace fixture (30 lines, 5.6KB) checked into `internal/classify/testdata/`, unlike the large `results_real` fixtures — small enough to version directly rather than gitignore, so `TestFailedTasks_RealFixture` runs unconditionally in CI rather than skipping when the fixture is absent.

**Narrator verified (2026-08-15):** run against the real `task_failed` finding, the narrator returned 4 claims — 2 `Reported` (the exit-137/SIGKILL mechanism and its OOM interpretation, both correctly cited to REASONING.md's rule candidate 3, not asserted from general training knowledge) and 2 `Observed` (the task's actual recorded duration, and the ruled-out-input-data check). No fabricated root cause, and no `Unknown` was needed here — unlike WT_REP1's outlier, this finding's mechanism is fully documented rather than genuinely unresolved, so the narrator correctly did not manufacture uncertainty where the evidence doesn't call for it. This closes the one open item from v3.

---

## 3. Open issues and future tasks

### 3.1 (Resolved 2026-07-30) `REASONING.md` restoration

Previously blocking: commit `565ed6e` had removed `REASONING.md` as an "internal planning doc," but it is a runtime input (`-reasoning` default, the only permitted source of `Reported` claims). Restored in commit `2ad4fc5` ("Restore REASONING.md, add classifier/narrator eval harness and scale tooling"). The README's usage examples correctly reference `REASONING.md`. No longer an open item — left here only so the history in §2 and the fix in git log line up for a reader.

### 3.2 Known limitation: concurrent cross-language writes untested

The replay proof ran Go and Python sequentially, about 35 seconds apart. This demonstrates that the shared log correctly interleaves events written by *different languages*, but not correct behavior under *simultaneous* writes from both at once. Already flagged honestly in `internal/eventlog`'s doc comment and the README. Per the project's own rules this is a candidate for work only if the limitation is actually hit — not a reason to build for it speculatively.

### 3.3 A second classifier rule

Only if it traces to a resolved mechanism. Rule candidate 2 (uniform FastQC `per_base_sequence_content` / `sequence_duplication_levels` failures) has two live, undistinguished hypotheses: that standard polyA-selected mRNA-seq libraries characteristically fail exactly these two modules (random-hexamer priming bias plus highly-expressed-transcript duplication), in which case it is expected background and must **never** fire as a rule; or a dataset/extraction-level confound. Resolving it needs either literature grounding in published FastQC guidance for RNA-seq, or a second independent real dataset. It stays out of the classifier until then.

### 3.4 (Resolved 2026-08-15) Real failure case

Previously the main gap: every pipeline run had succeeded, so every rule was a within-run outlier check, not a failure classifier. Resolved in §2.9 — `classify.FailedTasks` was verified against a real, deliberately induced STAR_ALIGN OOM kill. Caveat: this is one real failure, not a corpus of failures — the rule's generality (any `status == FAILED` task) rests on exit-code semantics being a general Nextflow/OS mechanism, not on having seen many failure instances, which is the same defensibility argument as AD-2's threshold, applied to a different kind of rule.

### 3.5 Smaller items

- Test coverage: `nftrace`, `multiqc`, `classify`, and `eval` all have tests now. `narrator`, `eventlog`, and `tracing` still have none.
- `WT_REP1`'s root cause remains genuinely Unknown — real candidates (biological wild-type variation in rRNA/contaminant load or RNA integrity, versus a batch/library-prep effect specific to this SRA run) are not distinguished by current data. This is a correct output, not a gap to close by guessing.
- Working-tree clutter (already gitignored, not a repo problem, but worth a local cleanup): `flowcast_events.db`, ~2MB of `.nextflow.log*` files, and `miniconda.sh` (a 155MB installer) sitting in the repo root.

---

## 4. Architecture decisions

Decisions are recorded with their real reason, including where the reason was pragmatic rather than technical.

### AD-1: Rule-based classifier, not ML

With n=5 samples there is nothing to train on, and a model that cannot explain itself defeats the purpose of a tool whose output is an evidence-cited explanation. A modified z-score is auditable line by line; the narrator can cite the exact median, MAD, and threshold that produced a flag.

### AD-2: Modified z-score (median + MAD), threshold 3.5

Chosen because it is an **established convention** (Iglewicz & Hoya 1993), not a value fit to this dataset. With 5 samples, a mean/standard-deviation z-score would let a single outlier inflate the standard deviation and mask itself; median and MAD are robust to that. The 3.5 threshold being conventional rather than tuned is the whole point — a tuned threshold on n=5 would be an invented number wearing a statistic's clothes.

Guards: return nil below 3 samples (no meaningful baseline), and nil when MAD is 0 (no spread to evaluate).

### AD-3: Confidence tags — Observed / Reported / Unknown

The central design commitment. `Observed` means computed from this run's own numbers; `Reported` means a documented mechanism cited to the reasoning document, not inferred and not from general training knowledge; `Unknown` is the honest answer when evidence doesn't support causation. Enforced in the system prompt and in a strict JSON schema with an enum, in both narrators.

`Unknown` is an expected, frequent, correct output. If the narrator stops saying it when it should, that is a regression, not an improvement.

### AD-4: Every classifier rule traces to the reasoning document

A rule that cannot point to a specific documented mechanism does not exist yet. This is what kept rule candidate 2 out of the classifier despite it being trivially easy to implement, and what makes the `Reported` tag mean something — the narrator is not free to source mechanisms from its own training data.

### AD-5: Standard library parsing

`encoding/csv` with a tab delimiter, and `encoding/json` decoding only the MultiQC subset actually used. The trace parser validates the 14-column header exactly and fails loudly on a mismatch rather than silently misaligning fields. Durations stay as Nextflow's raw strings (`"1m 30s"`) precisely because no rule reads them yet — parsing them would be speculative work.

### AD-6: Direct HTTP for the LLM API rather than a third-party Go SDK

`net/http` + `encoding/json` against the Chat Completions API, using strict structured JSON output (`json_schema`, `strict: true`). Avoids taking an unverified dependency for what amounts to one POST request, and stays consistent with the standard-library preference.

### AD-7: SQLite via `modernc.org/sqlite` (pure Go, no CGO)

Keeps `go build` and CI simple and cross-platform — no C toolchain, no CGO cross-compilation friction, which matters directly for the release workflow's static binary.

### AD-8: Shared event log as the cross-language contract

One SQLite file, one `events` table, both languages opening it with identical pragmas (WAL, 5s busy timeout). The contract between Go and Python is the *schema*, not an RPC interface or a shared library — which is why an independently implemented Python narrator could consume Go's findings with no coupling beyond the table definition.

Ordering: `ORDER BY ts ASC, id ASC`. `ts` is wall-clock from whichever process wrote the row; `id` is a table-wide autoincrement that tie-breaks same-timestamp rows in true insertion order. This is sound for the sequential-writer case it was proven under, and its untested case is documented (§3.2) rather than papered over. Note that wall-clock ordering across processes is only as good as the clock they share — a real concern if writers ever land on different machines.

### AD-9: LLM provider — OpenAI, for a non-technical reason

Both narrators call OpenAI (`gpt-4o`). Stated plainly: this was **not** a technical, cost, or capability decision. The v1 Claude narrator worked correctly and produced exactly the confidence-tagged output it was designed to produce (§2.6). The switch happened because only an OpenAI key was on hand when v2's real-run work needed credentials, and the tradeoff was explicitly chosen after being presented.

The narrator is a thin, provider-agnostic layer — system prompt plus strict JSON schema — so this is reversible. Notably, the OpenAI run returned 4 claims where Claude returned 13 from the same finding: different verbosity, same confidence-tagging behavior including the `Unknown`. That is a difference in models, not a regression.

The v1 record of the real Claude run stands as accurate history and was not rewritten.

### AD-10: v2 scope was a positioning decision, on the record

The project's Rule 1 says architecture only changes in response to a real bug or limitation hit while building. The v2 event log and Python SDK were not that — they were a deliberate portfolio-positioning decision by the project owner, recorded as an explicit override rather than quietly reinterpreted as a technical need.

That distinction is the honest version of the same discipline the narrator enforces: naming the actual reason instead of the flattering one.

### AD-11: Scope boundaries held

FlowCast is one narrow diagnostic layer. It does not compete with nf-prov / BCO / WRROC provenance capture or AWS HealthOmics, and having an event log does not change that — the differentiator is the honest narration layer; the log serves it. Rust, FFI, local model inference, and vector databases remain excluded. Docker is scoped to running the nf-core pipeline itself, never to FlowCast's own architecture.

---

## 5. Recommended order of work

1. Repo hygiene: `miniconda.sh`, `.nextflow.log*`, `flowcast_events.db` (§3.5).
2. Add tests for `internal/narrator` and `internal/eventlog` (§3.5).
3. Then, and only with a real reason: resolving rule candidate 2 (§3.3), or a second independent real dataset to test rule candidate 3's generality (§3.4).
