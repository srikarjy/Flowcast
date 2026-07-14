# FlowCast

FlowCast is a Go CLI that diagnoses nf-core/rnaseq pipeline runs. It parses Nextflow's `execution_trace.txt` and MultiQC's `multiqc_data.json`, runs a rule-based classifier over the real QC fields to flag outlier samples, and hands the findings to an LLM narrator that explains them — with every claim tagged by how well-supported it actually is.

## Why

Pipeline QC reports are full of numbers but short on narration. FlowCast's narrator doesn't get to invent a root cause just because it sounds plausible — every claim it makes is tagged:

- **Observed** — a fact computed directly from the run's own numbers.
- **Reported** — a documented mechanism, cited to its source, not inferred.
- **Unknown** — the honest answer when the evidence doesn't support a causal claim. This is a normal, expected output, not a failure of the tool.

## How it works

1. **Parse** — `internal/nftrace` reads the tab-separated Nextflow trace; `internal/multiqc` reads the relevant STAR alignment section out of MultiQC's JSON.
2. **Classify** — `internal/classify` runs a rule-based outlier check (a modified z-score over `unmapped_tooshort_percent` across samples) to flag samples that diverge from the run's own baseline.
3. **Narrate** — `internal/narrator` sends the classifier's findings, plus a written causal-reasoning reference, to an LLM (OpenAI, structured JSON output) and returns confidence-tagged claims.
4. **Event log + replay** — every stage can optionally emit events into a shared SQLite log (`internal/eventlog`), written to by both the Go pipeline and an independent Python narrator (`python/`). `flowcast replay` plays the whole log back, interleaved by timestamp, regardless of which language wrote which event.

## Usage

```bash
go build -o flowcast ./cmd/flowcast

# Run the pipeline: parse trace + MultiQC data, classify, narrate
./flowcast -trace execution_trace.txt -multiqc multiqc_data.json -reasoning reasoning.md

# Same, but also write every stage's events into a shared SQLite log
./flowcast -trace execution_trace.txt -multiqc multiqc_data.json -reasoning reasoning.md -eventlog events.db

# Replay the shared log, ordered by timestamp across languages
./flowcast replay -eventlog events.db
```

The narrator reads its API key from `OPENAI_API_KEY`.

### Python narrator

A second, independently implemented narrator lives in `python/`, reading the same shared event log:

```bash
pip install -r python/requirements.txt
python3 python/narrate.py --eventlog events.db --reasoning reasoning.md
```

## Event log ordering

The Go pipeline and Python narrator write into one shared SQLite file with an identical `events` table (WAL mode, so a process opening the file after another has written to it doesn't hit a locked database). `flowcast replay` orders rows by `ts ASC, id ASC` — `ts` is wall-clock time from whichever process wrote the row, and `id` is a table-wide autoincrement that reliably tie-breaks same-timestamp rows in true insertion order. This has been exercised with one writer active at a time (Go finishes, then Python runs); simultaneous concurrent writes from both languages have not been tested.

## Scope

Go, standard library parsing, a rule-based (not ML) classifier, and an LLM API with structured JSON output. No Rust, FFI, local inference, vector databases, or Docker in FlowCast's own architecture. This is one narrowly-scoped diagnostic layer, not a provenance-capture or pipeline-orchestration tool.

## Author

srikar jy
