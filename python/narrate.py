"""FlowCast's Python narrator.

Reads the real classifier `finding` events the Go pipeline already wrote into
the shared SQLite event log (internal/eventlog/eventlog.go), sends them plus
REASONING.md to an LLM (OpenAI — CLAUDE.md v3 scope amendment, 2026-07-13),
and writes the resulting confidence-tagged claims back into the same event
log as `source_lang=python` events. This is the actual cross-language proof
for `flowcast replay`: a second, independently implemented narrator, in a
different language, consuming and appending to the log a Go process wrote.

Every claim must carry a confidence_tag of Observed, Reported, or Unknown —
same rule as internal/narrator/narrator.go, CLAUDE.md Cardinal Rule 5.
"""

import argparse
import json
import sys

from openai import OpenAI

from flowcast_sdk import EventLog

MODEL = "gpt-4o"

SYSTEM_PROMPT = """You are FlowCast's failure narrator. You narrate nf-core/rnaseq pipeline QC classifier findings for a bioinformatics audience.

Rules, non-negotiable:
- Every claim you make must carry a confidence_tag: exactly one of "Observed", "Reported", or "Unknown".
- Observed: a fact computed directly from the numbers given to you below (the classifier finding, the QC values).
- Reported: a documented mechanism from the reasoning document given to you below — not something you infer, and not general training knowledge beyond what's given.
- Unknown: use this whenever a claim would require a causal explanation you have not been given evidence for. Do not guess at root cause. Unknown is a normal, expected, frequent answer — do not avoid it just to sound more confident.
- Never state or imply why a sample is an outlier unless the reasoning document explicitly establishes that cause for this data. If the reasoning document says the root cause is unresolved, output a claim with confidence_tag "Unknown" saying so explicitly — do not speculate (not biological, not technical, not batch-effect), even as a hedge.
- evidence_source must cite exactly where the claim comes from: a specific field name and value from the finding, or a specific section of the reasoning document. Never cite general knowledge.
- Output only claims supported by the material given to you. If there isn't enough to support a claim, output fewer claims rather than inventing one."""

OUTPUT_SCHEMA = {
    "type": "object",
    "properties": {
        "claims": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "claim": {"type": "string"},
                    "confidence_tag": {
                        "type": "string",
                        "enum": ["Observed", "Reported", "Unknown"],
                    },
                    "evidence_source": {"type": "string"},
                },
                "required": ["claim", "confidence_tag", "evidence_source"],
                "additionalProperties": False,
            },
        },
    },
    "required": ["claims"],
    "additionalProperties": False,
}


def build_user_message(findings: list[dict], reasoning_doc: str) -> str:
    lines = ["Classifier findings from this run:", ""]
    for f in findings:
        lines.append(
            f"- rule={f['rule']} sample={f['sample']} detail={f['detail']}"
        )
    lines.append("")
    lines.append("Causal reasoning document (REASONING.md) — the only source of Reported mechanisms:")
    lines.append("")
    lines.append(reasoning_doc)
    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--eventlog", required=True, help="path to the shared event log SQLite file")
    parser.add_argument("--reasoning", default="REASONING.md", help="path to the causal reasoning document")
    args = parser.parse_args()

    log = EventLog(args.eventlog)
    findings = log.findings()
    if not findings:
        print(f"no finding events in {args.eventlog}", file=sys.stderr)
        return 1

    with open(args.reasoning, encoding="utf-8") as f:
        reasoning_doc = f.read()

    client = OpenAI()
    response = client.chat.completions.create(
        model=MODEL,
        messages=[
            {"role": "system", "content": SYSTEM_PROMPT},
            {"role": "user", "content": build_user_message(findings, reasoning_doc)},
        ],
        response_format={
            "type": "json_schema",
            "json_schema": {
                "name": "narrator_claims",
                "strict": True,
                "schema": OUTPUT_SCHEMA,
            },
        },
    )

    raw_text = response.choices[0].message.content
    if raw_text is None:
        print("narrator: no content in response", file=sys.stderr)
        return 1

    claims = json.loads(raw_text)["claims"]

    print("python narrator claims:")
    for c in claims:
        print(f"  [{c['confidence_tag']}] {c['claim']} (source: {c['evidence_source']})")
        log.emit(
            "narrator",
            "claim",
            "",
            {
                "claim": c["claim"],
                "confidence_tag": c["confidence_tag"],
                "evidence_source": c["evidence_source"],
            },
        )

    log.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
