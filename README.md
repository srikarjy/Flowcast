# FlowCast

FlowCast is a Go CLI that diagnoses nf-core/rnaseq pipeline runs. It parses Nextflow's `execution_trace.txt` and MultiQC's `multiqc_data.json`, runs a rule-based classifier over the real trace and QC fields — outlier samples on healthy runs, and outright task failures — and hands the findings to an LLM narrator that explains them, with every claim tagged by how well-supported it actually is.

## About

FlowCast diagnoses nf-core/rnaseq runs with a rule-based classifier and an LLM narrator that tags every claim `Observed`, `Reported`, or `Unknown` — never a guessed root cause. Licensed under MIT; see `STATUS.md` for the full build history.

## My honest opinion on the project

**What's genuinely good:** the confidence-tagging discipline is the right idea, executed consistently rather than just claimed in the README. The project caught its own toy-data problem early (`STATUS.md` §2.2) instead of building a classifier on noise, and it deliberately left a second outlier-rule candidate (uniform FastQC failures) unimplemented because the mechanism wasn't resolved — that's a harder discipline to hold than it sounds, especially when the easy thing would be to ship the rule anyway. Two independently implemented narrators (Go/OpenAI and Python) converging on the same confidence tags from the same evidence is real, checkable evidence that the tagging lives in the prompt and reasoning document, not in one model's quirks. The task-failure rule closes what was previously the project's biggest real gap: every earlier run had succeeded, so nothing had actually been tested against a failure. It has now — verified end to end, narrator included, against a real induced OOM kill.

**What's overbuilt relative to what's proven:** the event log + cross-language replay is honestly labeled in `STATUS.md` (AD-10) as a portfolio-positioning decision, not something a real bug or limitation forced — and that labeling is correct. As infrastructure it's more sophisticated than the two classifier rules it currently serves. Fine as a demonstration of engineering range, not yet load-bearing for the tool's stated purpose.

**What's still thin:** both classifier rules rest on a single real run each — the outlier rule on n=5 samples, the failure rule on one induced failure. Both are defensible as general, established mechanisms (a robust-statistics convention; SIGKILL/exit-137 semantics) rather than values tuned to one dataset, but neither has been tested against a second independent dataset yet, so I wouldn't over-trust either rule's generality until it has.

## Why

Pipeline QC reports are full of numbers but short on narration. FlowCast's narrator doesn't get to invent a root cause just because it sounds plausible — every claim it makes is tagged:

- **Observed** — a fact computed directly from the run's own numbers.
- **Reported** — a documented mechanism, cited to its source, not inferred.
- **Unknown** — the honest answer when the evidence doesn't support a causal claim. This is a normal, expected output, not a failure of the tool.

## How it works

1. **Parse** — `internal/nftrace` reads the tab-separated Nextflow trace; `internal/multiqc` reads the relevant STAR alignment section out of MultiQC's JSON (optional — see below).
2. **Classify** — `internal/classify` runs two rules: `FailedTasks` flags any trace task with status `FAILED` (no baseline needed — a single real failure is itself the finding), and `UnmappedTooShortOutliers` runs a modified z-score over `unmapped_tooshort_percent` across samples to flag within-run outliers on otherwise-healthy data.
3. **Narrate** — `internal/narrator` sends the classifier's findings, plus a written causal-reasoning reference, to an LLM (OpenAI, structured JSON output) and returns confidence-tagged claims.
4. **Event log + replay** — every stage can optionally emit events into a shared SQLite log (`internal/eventlog`), written to by both the Go pipeline and an independent Python narrator (`python/`). `flowcast replay` plays the whole log back, interleaved by timestamp, regardless of which language wrote which event.

## Usage

```bash
go build -o flowcast ./cmd/flowcast

# Run the pipeline: parse trace + MultiQC data, classify, narrate
./flowcast -trace execution_trace.txt -multiqc multiqc_data.json -reasoning REASONING.md

# -multiqc is optional: a failed run may never produce MultiQC output for the
# failed sample, so the FAILED-task rule works off the trace alone
./flowcast -trace execution_trace.txt -reasoning REASONING.md

# Same, but also write every stage's events into a shared SQLite log
./flowcast -trace execution_trace.txt -multiqc multiqc_data.json -reasoning REASONING.md -eventlog events.db

# Replay the shared log, ordered by timestamp across languages
./flowcast replay -eventlog events.db

# Synthetic classifier benchmark (bootstrapped from real baseline values, honestly labeled — see internal/eval)
./flowcast eval -trials 2000 -seed 1

# Live narrator ablation: strict vs. naive prompt, unsupported-claim rate (calls OpenAI, billed to OPENAI_API_KEY)
./flowcast eval-narrator -trace execution_trace.txt -multiqc multiqc_data.json -reasoning REASONING.md

# Combined scale counts across one or more real runs (comma-separated, matched by position)
./flowcast stats -trace t1.txt,t2.txt -multiqc m1.json,m2.json
```

The narrator reads its API key from `OPENAI_API_KEY`.

### Python narrator

A second, independently implemented narrator lives in `python/`, reading the same shared event log:

```bash
pip install -r python/requirements.txt
python3 python/narrate.py --eventlog events.db --reasoning REASONING.md
```

## Event log ordering

The Go pipeline and Python narrator write into one shared SQLite file with an identical `events` table (WAL mode, so a process opening the file after another has written to it doesn't hit a locked database). `flowcast replay` orders rows by `ts ASC, id ASC` — `ts` is wall-clock time from whichever process wrote the row, and `id` is a table-wide autoincrement that reliably tie-breaks same-timestamp rows in true insertion order. This has been exercised with one writer active at a time (Go finishes, then Python runs); simultaneous concurrent writes from both languages have not been tested.

## Scope

Go, standard library parsing, a rule-based (not ML) classifier, and an LLM API with structured JSON output. No Rust, FFI, local inference, vector databases, or Docker in FlowCast's own architecture. This is one narrowly-scoped diagnostic layer, not a provenance-capture or pipeline-orchestration tool.

## Author

srikar jy
