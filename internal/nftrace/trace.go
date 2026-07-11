// Package nftrace parses Nextflow's tab-separated execution_trace.txt.
package nftrace

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
)

// Task is one row of execution_trace.txt. Duration/Realtime are kept as
// Nextflow's raw formatted strings (e.g. "23s", "1m 30s") rather than
// parsed into time.Duration, since no classifier rule reads them yet.
type Task struct {
	TaskID     string
	Hash       string
	NativeID   string
	Name       string
	Status     string
	Exit       string
	Submit     string
	Duration   string
	Realtime   string
	CPUPercent string
	PeakRSS    string
	PeakVMem   string
	RChar      string
	WChar      string
}

var columns = []string{
	"task_id", "hash", "native_id", "name", "status", "exit", "submit",
	"duration", "realtime", "%cpu", "peak_rss", "peak_vmem", "rchar", "wchar",
}

// LoadTasks reads a Nextflow execution_trace.txt file and returns one Task
// per row, in file order.
func LoadTasks(path string) ([]Task, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("nftrace: open %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("nftrace: read header of %s: %w", path, err)
	}
	if err := checkHeader(header); err != nil {
		return nil, fmt.Errorf("nftrace: %s: %w", path, err)
	}

	var tasks []Task
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("nftrace: read row of %s: %w", path, err)
		}
		tasks = append(tasks, Task{
			TaskID: row[0], Hash: row[1], NativeID: row[2], Name: row[3],
			Status: row[4], Exit: row[5], Submit: row[6], Duration: row[7],
			Realtime: row[8], CPUPercent: row[9], PeakRSS: row[10],
			PeakVMem: row[11], RChar: row[12], WChar: row[13],
		})
	}
	return tasks, nil
}

func checkHeader(got []string) error {
	if len(got) != len(columns) {
		return fmt.Errorf("expected %d columns, got %d: %v", len(columns), len(got), got)
	}
	for i, want := range columns {
		if got[i] != want {
			return fmt.Errorf("column %d: expected %q, got %q", i, want, got[i])
		}
	}
	return nil
}
