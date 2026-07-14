# FlowCast — Cardinal Rules

These rules are locked. Do not revisit, redesign, or expand scope without explicitly flagging the specific rule being broken and why.

## 1. No new architecture without a working failure first
The only valid reason to change the design is a real bug or limitation hit while building, not a new idea that sounds more impressive on paper.

## 2. Nothing claimed unless it's actually running on real data
No metric, no percentage, no "it detects X" goes into code comments, README, commit messages, or anywhere else unless it came out of an actual run against a real `trace.txt` or `multiqc_data.json`. Not estimated. Not "should work."

## 3. Stack is frozen for v1
Go, standard library parsing only, rule based classifier (not ML), Claude API with structured JSON output. Rust, FFI, local inference, vector databases, and Docker are not up for debate for v1. If one becomes genuinely necessary later, it needs a real measured reason (an actual latency number that's actually a problem), not a preference.

## 4. Every classifier rule traces back to the reasoning document
If a rule can't be traced to a specific line in the one page causal biology/technical reasoning document, the rule doesn't exist yet. No inventing thresholds.

## 5. Every narrator claim carries a confidence tag, and "Unknown" is a valid, expected, frequent output
The narrator must never guess at causation it hasn't measured. If it stops saying Unknown when it should, that's a regression, not progress.

## 6. One real end to end run before any supporting infrastructure
No eval harness, no CI, no OpenTelemetry, no observability work until one full real diagnosis exists on real data. Building infrastructure around something unproven is how this kind of project dies quietly.

## 7. This project is one signal, not a silver bullet
FlowCast does not get to be "the project that gets every biotech callback." It's one well built, narrow, honestly scoped piece of a larger portfolio. Keep it scoped to what it actually is.

## v1 Locked Scope (for reference)
A Go CLI that parses Nextflow's `trace.txt` and MultiQC's `multiqc_data.json` from one real nf-core/rnaseq run, applies a rule based failure classifier built from real QC fields, and feeds a Claude API narrator (structured JSON output: claim, confidence_tag, evidence_source) that only makes Observed/Reported/Unknown tagged claims and refuses unmeasured causal claims.

**Explicitly not competing with:** nf-prov / BCO / WRROC provenance capture, or AWS HealthOmics. FlowCast's differentiator is the honest, confidence tagged failure narration layer, not provenance capture itself.

**Explicitly excluded from v1:** Rust, FFI, local model inference, weblog live streaming, vector DB, Docker, resource prediction, recommendation engine.

## After v1 ships
Once v1 is complete and demonstrably working end to end on real data, scope for further additions (beyond the v1 exclusions above) can be discussed — but only after v1 is done, and only following the same rules above (real failure first, real data, traceable rules, confidence tags, no premature infrastructure).

## v2 Scope Amendment (2026-07-13)

v1 is complete: real trace.txt + multiqc_data.json parsed, Rule 1 classifier verified against real data, and a real end-to-end narrator run completed with confidence-tagged claims including a real `Unknown` (see `PROGRESS.md` §8). Rule 6's gate is satisfied.

**This amendment is an explicit, on-the-record exception to Rule 1**, not a silent violation of it. The honest reason for v2's scope, stated plainly: this is a deliberate hiring/portfolio positioning decision (see `PORTFOLIO_REVIEW (1).md`), made knowingly by the project owner — not a bug or limitation hit while building v1. Rule 1 as originally written would forbid this; it is being overridden here by explicit owner decision, not reinterpreted or worked around.

**Rule 3 (frozen stack) is amended for v2 only:** SQLite is added, as an event log / collector store. A Python SDK is added, to emit events from the narrator side and to support a `replay` command that interleaves Go and Python events. Rust, FFI, local model inference, and vector databases remain excluded — this amendment does not reopen those. Docker remains scoped to running the nf-core pipeline itself, not FlowCast's own architecture.

**Everything else stays in force for v2:**
- Rule 2 (nothing claimed unless it ran on real data) — the event log and replay must be demonstrated against the real `WT_REP1` run, not asserted as working.
- Rule 4 (every classifier rule traces to `REASONING.md`) — a second classifier rule still needs a resolved, traceable reasoning entry; it cannot be invented to "prove the rule engine generalizes."
- Rule 5 (confidence tags, Unknown is expected) — unchanged.
- Rule 7 (one signal, not a silver bullet) — unchanged; v2 does not get to claim it competes with nf-prov/BCO/WRROC or AWS HealthOmics provenance capture just because it now has an event log. The differentiator is still the honest narration layer; the event log serves that, it doesn't replace it.

## v2 Locked Scope (for reference)
Phase 2 from `PORTFOLIO_REVIEW (1).md`: SQLite schema + collector, Go emitter wired into the existing classifier/narrator pipeline, a `replay` command proven Go-only first, then a Python SDK + narrator wired in, then an interleaved Go+Python replay proof on the real `WT_REP1` run, then a README section on ordering tradeoffs. Basic CI (`go test ./...` on push) is now unblocked by Rule 6 and can proceed alongside this.

## v3 Scope Amendment (2026-07-13): LLM provider switched from Claude API to OpenAI API

**Rule 3 is amended again — the "Claude API with structured JSON output" line is replaced with "an LLM API with structured JSON output (OpenAI)."** This applies to both the Go narrator (`internal/narrator`) and the Python narrator (`python/narrate.py`).

**Honest reason, stated plainly (per Rule 1's own requirement to flag what's being broken and why):** this was not a technical limitation of the Claude API — the v1 Claude narrator run (§8 in `PROGRESS.md`) worked correctly and produced exactly the confidence-tagged output it was designed to produce. The reason for this switch is that the project owner only had an OpenAI API key on hand at the time the v2 real-run work needed credentials, and explicitly chose, twice, after being shown the tradeoff (including the option to wait for a Claude key, and the option to keep Go on Claude and only switch Python), to switch both narrators to OpenAI permanently. This is a credential-availability decision, not a capability or cost or architecture decision.

**What this does NOT change:**
- Rule 5 (every claim needs a confidence tag; Unknown is expected and frequent) — unchanged, enforced in the OpenAI system prompt exactly as it was for Claude.
- Rule 4 (classifier rules trace to `REASONING.md`) — unchanged; the classifier itself doesn't call any LLM.
- Rule 2 (nothing claimed unless real) — the OpenAI narrator run must be real, on real data, before anything about its behavior is claimed in `PROGRESS.md`.
- The **historical v1 record stays accurate**: PROGRESS.md §8 documents a real Claude narrator run that actually happened on 2026-07-13, before this amendment. That record is not rewritten or retracted — it's what was true at the time. Going forward from this amendment, new real runs use OpenAI.

**Implementation note:** the Go narrator calls OpenAI's Chat Completions API via `net/http` + `encoding/json` (standard library) rather than an unverified third-party Go SDK, consistent with Rule 3's original "standard library" preference. The Python narrator uses the official `openai` Python package.
