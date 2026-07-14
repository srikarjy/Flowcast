# FlowCast — Portfolio Readiness Review & Completion Plan

_Last updated: 2026-07-13_

## Scoring (current state, based on what's actually been verified)

**1. Skill Score: 6/10**
You've shown real Go, real Nextflow/nf-core operation, real MultiQC/trace
parsing, and a working Claude API integration. That's legitimate range.
What's missing: no tests visible yet, no CI, and the "distributed tracing"
claim isn't real yet — right now it's two programs that each print to
stdout, not a system that traces causality across a boundary.

**2. Hiring Signal Score: 5/10**
As of today, this reads as "I can write a CLI that calls an LLM API and
parses bioinformatics output." That's a decent signal for a
bioinformatics-adjacent backend role. It is not yet a signal for
"systems/infra engineer who understands distributed tracing," which is the
differentiated positioning you're actually going for. The event log is what
closes that gap — until it exists, the pitch is aspirational.

**3. Scientific Depth Score: 6/10**
Rule 1's reasoning (modified z-score, MAD over mean, ruling out mismatch
rate/multimapping/trimming/truncation as competing causes) is genuinely
good epistemic hygiene — better than most bioinformatics portfolio
projects, which just run a pipeline and paste a screenshot. But it's one
rule. A recruiter who knows RNA-seq will ask "what about STAR
unique-mapping rate anomalies, or ribosomal contamination, or 3' bias?" and
right now you have one flagged failure mode.

**4. Engineering Depth Score: 5/10**
Clean package boundaries (`multiqc`, `nftrace`, `classify`), that's real.
But no tests mentioned anywhere in this conversation, no error handling
shown beyond the happy path, and the event log / SQLite collector is still
just a design doc, not code. Depth is claimed in docs, not yet demonstrated
in a repo a stranger can `git clone` and evaluate in 10 minutes.

**5. Uniqueness Score: 7/10**
This is the strongest axis. "Provenance-tracked pipeline diagnosis with
cross-language distributed tracing" is not a project category seen often.
Most portfolio bioinformatics projects are "I ran nf-core and made a
Streamlit dashboard." If the event log and replay actually ship, this
becomes memorable in a stack of otherwise identical portfolios.

**6. Probability It Impresses Hiring Managers: 35% today, could reach 70%+
if finished properly.**
Today it impresses someone who reads the README carefully. Most hiring
managers give a repo 90 seconds. Right now those 90 seconds show a CLI
tool, not a distributed system. The gap between what's built and what's
compelling is entirely in finishing Phase 2, adding tests, and rewriting
the README so the differentiator is visible in the first paragraph, not
buried in prose.

**7. What Would Make It Better (concrete):**
- Finish the event log + replay (Phase 2, already scoped) — this is
  non-negotiable, it's the entire thesis of the project.
- Add a second classifier rule so "Rule 1" doesn't read as the whole
  system. Even one more (e.g. ribosomal RNA contamination via
  `rRNA_percent` in MultiQC, or a duplication-rate outlier) proves the rule
  engine generalizes.
- Add Go tests for the classifier (table-driven, using synthetic MultiQC
  JSON fixtures) — 0 tests currently visible is the single biggest red
  flag for a "systems engineer" pitch.
- Rewrite the README's first three lines to lead with: "diagnoses
  nf-core/rnaseq pipeline anomalies and produces auditable, source-cited
  reasoning traced across Go and Python, ordered by a documented causal
  model" — lead with the differentiator, not the tech stack.
- A 90-second demo GIF/asciinema of the `replay` command showing
  interleaved Go+Python events. Hiring managers don't read code, they
  watch demos.

**8. Next Skill To Learn: SQLite transaction semantics and write-ahead
logging** — since the collector will be under concurrent writes from two
languages, and it's the one part of Phase 2 where "it worked" and "it
works correctly" diverge in a way a serious interviewer would actually
probe.

## Full Completion Plan to Make This Portfolio-Ready

### Phase 2 (already scoped) — the core differentiator
1. SQLite schema + collector endpoint
2. Go emitter wired in
3. `replay` command, Go-only proof
4. Python SDK + narrator wired in
5. Interleaved replay proof on the `WT_REP1` run
6. README ordering-tradeoffs section

### Phase 3 — credibility hardening (not currently scoped, but load-bearing)
- Table-driven tests for the `classify` package (synthetic
  outlier/non-outlier fixtures)
- A second classifier rule, written with the same REASONING.md-first
  discipline as Rule 1
- Basic CI (GitHub Actions: `go test ./...` on push) — five minutes of
  work, disproportionate signal
- Error handling for malformed/missing MultiQC or trace files (right now
  it's unclear what happens on bad input — that's a real gap an
  interviewer will poke at)

### Phase 4 — packaging for hiring managers
- README rewrite leading with the differentiator, not the file structure
- Architecture diagram (Go parser → classifier → event log ← Python
  narrator → replay) — one image, worth more than three paragraphs
- Short demo capture (asciinema or GIF) of `replay` output
- Public repo, clean commit history (squash the debugging churn — 5
  pipeline runs in `pipeline_info` is fine locally, don't let that mess
  leak into the git history narrative)

## Recommended Immediate Next Step

Given zero tests is the biggest current liability, do the Go test suite for
the existing classifier **before** the second classifier rule. Tests first,
then a second rule that the tests already have a harness to validate.
