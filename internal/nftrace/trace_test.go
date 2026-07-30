package nftrace

import (
	"os"
	"path/filepath"
	"testing"
)

// realTracePath is FlowCast's canonical real-data baseline (STATUS.md §2.4).
// results_real/ is gitignored (large pipeline output), so this test skips
// rather than fails when it isn't present locally, e.g. in CI.
const realTracePath = "../../results_real/pipeline_info/execution_trace_2026-07-09_23-42-27.txt"

func TestLoadTasks_RealFixture(t *testing.T) {
	if _, err := os.Stat(realTracePath); err != nil {
		t.Skipf("real fixture not present (gitignored): %v", err)
	}

	tasks, err := LoadTasks(realTracePath)
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}

	const wantTasks = 208
	if len(tasks) != wantTasks {
		t.Fatalf("expected %d tasks, got %d", wantTasks, len(tasks))
	}

	for _, task := range tasks {
		if task.Status != "COMPLETED" && task.Status != "CACHED" {
			t.Errorf("task %s (%s): unexpected status %q, want COMPLETED or CACHED", task.TaskID, task.Name, task.Status)
		}
	}
}

func TestLoadTasks_MalformedHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad_trace.txt")
	if err := os.WriteFile(path, []byte("task_id\thash\tname\n1\tabc\tfoo\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if _, err := LoadTasks(path); err == nil {
		t.Fatal("expected error for malformed header, got nil")
	}
}

func TestLoadTasks_MissingFile(t *testing.T) {
	if _, err := LoadTasks(filepath.Join(t.TempDir(), "does_not_exist.txt")); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
