"""Python client for FlowCast's shared SQLite event log.

Writes into the identical `events` table the Go side defines
(internal/eventlog/eventlog.go) — same columns, same WAL pragma — so a
Python process can append to a log a Go process already wrote, and
`flowcast replay` can interleave both by timestamp (v2 scope amendment,
CLAUDE.md).
"""

from __future__ import annotations

import json
import sqlite3
from datetime import datetime, timezone

_SCHEMA = """
CREATE TABLE IF NOT EXISTS events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          TEXT NOT NULL,
    source_lang TEXT NOT NULL,
    component   TEXT NOT NULL,
    event_type  TEXT NOT NULL,
    sample      TEXT,
    payload     TEXT NOT NULL
);
"""


class EventLog:
    """Shared event log handle. Same file/schema the Go pipeline writes."""

    def __init__(self, path: str):
        self._conn = sqlite3.connect(path, timeout=5.0)
        self._conn.execute("PRAGMA journal_mode=WAL;")
        self._conn.execute("PRAGMA busy_timeout=5000;")
        self._conn.execute(_SCHEMA)
        self._conn.commit()

    def emit(self, component: str, event_type: str, sample: str, payload: dict) -> None:
        ts = datetime.now(timezone.utc).isoformat(timespec="microseconds").replace("+00:00", "Z")
        self._conn.execute(
            "INSERT INTO events (ts, source_lang, component, event_type, sample, payload) "
            "VALUES (?, 'python', ?, ?, ?, ?)",
            (ts, component, event_type, sample or None, json.dumps(payload)),
        )
        self._conn.commit()

    def findings(self, component: str = "classify", event_type: str = "finding") -> list[dict]:
        """Read back finding events written by the Go classifier stage."""
        rows = self._conn.execute(
            "SELECT sample, payload FROM events WHERE component = ? AND event_type = ? "
            "AND source_lang = 'go' ORDER BY ts ASC, id ASC",
            (component, event_type),
        ).fetchall()
        out = []
        for sample, payload in rows:
            record = json.loads(payload)
            record["sample"] = sample
            out.append(record)
        return out

    def close(self) -> None:
        self._conn.close()

    def __enter__(self) -> "EventLog":
        return self

    def __exit__(self, *exc_info) -> None:
        self.close()
