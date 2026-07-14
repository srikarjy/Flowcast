// Command flowcast parses a real Nextflow execution trace and MultiQC
// data.json, and runs FlowCast's rule-based classifier against them.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"

	"flowcast/internal/classify"
	"flowcast/internal/eventlog"
	"flowcast/internal/multiqc"
	"flowcast/internal/narrator"
	"flowcast/internal/nftrace"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "replay" {
		runReplay(os.Args[2:])
		return
	}
	runPipeline(os.Args[1:])
}

// runReplay implements `flowcast replay -eventlog <path>`: prints every
// event in the shared event log, ordered by timestamp, regardless of which
// language wrote it (v2 scope amendment, CLAUDE.md).
func runReplay(args []string) {
	fs := flag.NewFlagSet("replay", flag.ExitOnError)
	eventlogPath := fs.String("eventlog", "", "path to the shared event log SQLite file")
	fs.Parse(args)

	if *eventlogPath == "" {
		fmt.Fprintln(os.Stderr, "usage: flowcast replay -eventlog <path>")
		os.Exit(2)
	}

	events, err := eventlog.List(*eventlogPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if len(events) == 0 {
		fmt.Println("no events in", *eventlogPath)
		return
	}
	for _, e := range events {
		sample := e.Sample
		if sample == "" {
			sample = "-"
		}
		fmt.Printf("[%s] [%-6s] %-10s %-14s %-20s %s\n", e.Ts, e.SourceLang, e.Component, e.EventType, sample, e.Payload)
	}
}

// runPipeline is the original `flowcast -trace ... -multiqc ...` pipeline,
// unchanged in default behavior, with an added optional -eventlog flag.
func runPipeline(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	tracePath := fs.String("trace", "", "path to Nextflow execution_trace.txt")
	multiqcPath := fs.String("multiqc", "", "path to MultiQC multiqc_data.json")
	reasoningPath := fs.String("reasoning", "REASONING.md", "path to the causal reasoning document")
	eventlogPath := fs.String("eventlog", "", "optional path to a shared event log SQLite file to emit events into")
	fs.Parse(args)

	if *tracePath == "" || *multiqcPath == "" {
		fmt.Fprintln(os.Stderr, "usage: flowcast -trace <execution_trace.txt> -multiqc <multiqc_data.json>")
		os.Exit(2)
	}

	var elog *eventlog.DB
	if *eventlogPath != "" {
		var err error
		elog, err = eventlog.Open(*eventlogPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		defer elog.Close()
	}

	tasks, err := nftrace.LoadTasks(*tracePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("parsed %d tasks from %s\n", len(tasks), *tracePath)

	nonCompleted := 0
	for _, t := range tasks {
		if t.Status != "COMPLETED" && t.Status != "CACHED" {
			nonCompleted++
			fmt.Printf("  task %s (%s): status=%s exit=%s\n", t.TaskID, t.Name, t.Status, t.Exit)
		}
	}
	if nonCompleted == 0 {
		fmt.Println("  all tasks COMPLETED or CACHED")
	}
	if elog != nil {
		if err := elog.Emit("nftrace", "trace_parsed", "", map[string]any{
			"trace_path": *tracePath, "task_count": len(tasks), "non_completed": nonCompleted,
		}); err != nil {
			fmt.Fprintln(os.Stderr, "eventlog error:", err)
		}
	}

	stars, err := multiqc.LoadStarStats(*multiqcPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("\nparsed STAR stats for %d samples from %s\n", len(stars), *multiqcPath)

	findings := classify.UnmappedTooShortOutliers(stars)
	if len(findings) == 0 {
		fmt.Println("classifier: no findings")
		return
	}

	sort.Slice(findings, func(i, j int) bool { return findings[i].Sample < findings[j].Sample })
	fmt.Println("classifier findings:")
	for _, f := range findings {
		fmt.Printf("  [%s] %s: %s\n", f.Rule, f.Sample, f.Detail)
		if elog != nil {
			if err := elog.Emit("classify", "finding", f.Sample, map[string]any{"rule": f.Rule, "detail": f.Detail}); err != nil {
				fmt.Fprintln(os.Stderr, "eventlog error:", err)
			}
		}
	}

	reasoningDoc, err := os.ReadFile(*reasoningPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nskipping narrator: could not read reasoning doc: %v\n", err)
		return
	}

	fmt.Println("\nnarrator claims:")
	claims, err := narrator.Narrate(context.Background(), findings, string(reasoningDoc))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	for _, c := range claims {
		fmt.Printf("  [%s] %s (source: %s)\n", c.ConfidenceTag, c.Claim, c.EvidenceSource)
		if elog != nil {
			if err := elog.Emit("narrator", "claim", "", map[string]any{
				"claim": c.Claim, "confidence_tag": c.ConfidenceTag, "evidence_source": c.EvidenceSource,
			}); err != nil {
				fmt.Fprintln(os.Stderr, "eventlog error:", err)
			}
		}
	}
}
