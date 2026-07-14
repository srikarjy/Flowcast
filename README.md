# FlowCast

A Go CLI that parses Nextflow's `trace.txt` and MultiQC's `multiqc_data.json`, applies a rule-based failure classifier built from real QC fields, and feeds an LLM narrator that only makes `Observed`/`Reported`/`Unknown`-tagged claims. See `CLAUDE.md` for the project's locked scope and rules, and `PROGRESS.md` for what's actually been built and verified against real data.

## Event log ordering tradeoffs

FlowCast's Go pipeline and Python narrator both write into one shared SQLite file (`internal/eventlog/eventlog.go`, `python/flowcast_sdk/eventlog.py`) with an identical `events` table, and `flowcast replay -eventlog <path>` lists every row `ORDER BY ts ASC, id ASC`, regardless of which language wrote it. This was proven against a real run — see `PROGRESS.md` §9.

**How ordering actually works, and its real limitation:**

- `ts` is wall-clock time, generated independently by each process (`time.Now().UTC()` in Go, `datetime.now(timezone.utc)` in Python) at the moment it inserts a row — there is no shared clock authority or vector clock coordinating the two languages. On one machine (the only case exercised so far) this is fine; across two machines it would only be as reliable as clock sync between them.
- `id` is `INTEGER PRIMARY KEY AUTOINCREMENT`, global to the table regardless of which process's connection inserts a row, so it's a correct tiebreaker for same-timestamp rows in the order they actually landed in the file.
- WAL mode + a 5-second `busy_timeout` let a process open the file after another has written to it without hitting a locked database, and let a process that finds it briefly locked wait instead of failing.

**What the real §9 run did and didn't prove:** the Go pipeline ran to completion first (last event `02:19:39.1275Z`), and the Python narrator ran roughly 35 seconds later (first event `02:20:14.854Z`), reading back Go's `finding` event and appending its own `claim` events. `flowcast replay` correctly interleaved both languages' events by timestamp afterward. That's real proof that the shared log and timestamp-ordered replay work *across* languages — it is not proof that ordering holds up under *simultaneous* concurrent writes from both languages at once, which has never been exercised. If that's ever actually hit as a real limitation (Cardinal Rule 1: real bug first, not a hypothetical), it would need to be revisited then — not designed around now.
