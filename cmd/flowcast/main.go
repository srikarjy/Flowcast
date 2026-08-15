// Command flowcast parses a real Nextflow execution trace and MultiQC
// data.json, and runs FlowCast's rule-based classifier against them.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"flowcast/internal/classify"
	"flowcast/internal/eval"
	"flowcast/internal/eventlog"
	"flowcast/internal/multiqc"
	"flowcast/internal/narrator"
	"flowcast/internal/nftrace"
	"flowcast/internal/tracing"
)

func main() {
	// Initialize OpenTelemetry tracing
	shutdown, err := tracing.InitTracer("flowcast")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize tracing: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "error shutting down tracer: %v\n", err)
		}
	}()

	if len(os.Args) > 1 && os.Args[1] == "replay" {
		runReplay(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "eval" {
		runEval(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "eval-narrator" {
		runEvalNarrator(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "stats" {
		runStats(os.Args[2:])
		return
	}
	runPipeline(os.Args[1:])
}

// runStats implements `flowcast stats`: given one or more real
// trace/multiqc pairs (comma-separated, matched by position), prints
// combined scale counts across all of them — real, growable N runs / M
// task records / K QC metrics numbers, as more real pipeline runs are
// added. Does not classify or narrate: each run's classifier baseline is
// specific to that run's own samples, so runs are not merged for that
// purpose, only counted.
func runStats(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	tracePaths := fs.String("trace", "", "comma-separated paths to execution_trace.txt files, one per run")
	multiqcPaths := fs.String("multiqc", "", "comma-separated paths to multiqc_data.json files, one per run")
	fs.Parse(args)

	traces := splitNonEmpty(*tracePaths)
	multiqcs := splitNonEmpty(*multiqcPaths)
	if len(traces) == 0 || len(multiqcs) == 0 || len(traces) != len(multiqcs) {
		fmt.Fprintln(os.Stderr, "usage: flowcast stats -trace <t1.txt,t2.txt,...> -multiqc <m1.json,m2.json,...> (same count, matched by position)")
		os.Exit(2)
	}

	totalTasks, totalSamples, totalMetrics := 0, 0, 0
	for i := range traces {
		tasks, err := nftrace.LoadTasks(traces[i])
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		stars, err := multiqc.LoadStarStats(multiqcs[i])
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		metricsThisRun := len(stars) * numStarFields
		fmt.Printf("run %d: %s -> %d tasks; %s -> %d samples (%d QC metrics)\n",
			i+1, traces[i], len(tasks), multiqcs[i], len(stars), metricsThisRun)

		totalTasks += len(tasks)
		totalSamples += len(stars)
		totalMetrics += metricsThisRun
	}

	fmt.Println()
	fmt.Printf("combined: %d real pipeline runs, %d task records, %d QC samples, %d QC metrics parsed\n",
		len(traces), totalTasks, totalSamples, totalMetrics)
}

// numStarFields is the count of fields in multiqc.StarSample — each parsed
// sample contributes this many individual QC metric values.
const numStarFields = 6

func splitNonEmpty(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// runEvalNarrator implements `flowcast eval-narrator`: runs the strict vs.
// naive narrator ablation (internal/eval.RunNarratorAblation) against a
// real trace/multiqc pair and prints the unsupported-claim comparison.
// Makes real OpenAI API calls billed to OPENAI_API_KEY.
func runEvalNarrator(args []string) {
	fs := flag.NewFlagSet("eval-narrator", flag.ExitOnError)
	tracePath := fs.String("trace", "", "path to Nextflow execution_trace.txt")
	multiqcPath := fs.String("multiqc", "", "path to MultiQC multiqc_data.json")
	reasoningPath := fs.String("reasoning", "REASONING.md", "path to the causal reasoning document")
	fs.Parse(args)

	if *tracePath == "" || *multiqcPath == "" {
		fmt.Fprintln(os.Stderr, "usage: flowcast eval-narrator -trace <execution_trace.txt> -multiqc <multiqc_data.json>")
		os.Exit(2)
	}

	if _, err := nftrace.LoadTasks(*tracePath); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	stars, err := multiqc.LoadStarStats(*multiqcPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	findings := classify.UnmappedTooShortOutliers(stars)
	if len(findings) == 0 {
		fmt.Println("no classifier findings on this run; nothing to narrate")
		return
	}

	reasoningDoc, err := os.ReadFile(*reasoningPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Println("Calling OpenAI twice (strict + naive narrator) against real findings — billed to OPENAI_API_KEY...")
	result, err := eval.RunNarratorAblation(context.Background(), findings, string(reasoningDoc))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("Narrator ablation — SYNTHETIC labels are not used here; findings and claims are real API output.")
	fmt.Println("Unsupported = causal claim without a citable evidence string (heuristic, see internal/eval.ScoreUnsupported).")
	fmt.Println()
	fmt.Printf("strict (Narrate):      %d/%d unsupported\n", result.StrictUnsupported, result.StrictTotal)
	fmt.Printf("naive (NarrateNaive):  %d/%d unsupported\n", result.NaiveUnsupported, result.NaiveTotal)
}

// runEval implements `flowcast eval`: runs the synthetic classifier
// benchmark (internal/eval) and prints its methodology + results report.
func runEval(args []string) {
	fs := flag.NewFlagSet("eval", flag.ExitOnError)
	trials := fs.Int("trials", 2000, "number of synthetic trials to run")
	seed := fs.Int64("seed", 1, "random seed, for a reproducible report")
	fs.Parse(args)

	report := eval.Evaluate(*trials, *seed)
	fmt.Print(report.String())
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
	ctx := context.Background()
	ctx, span := tracing.StartSpan(ctx, "flowcast.pipeline")
	defer span.End()

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

	// Trace: load Nextflow trace
	ctx, span = tracing.StartSpan(ctx, "load_nftrace")
	tasks, err := nftrace.LoadTasks(*tracePath)
	span.End()
	if err != nil {
		tracing.RecordError(ctx, err)
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

	// Trace: load MultiQC stats
	ctx, span = tracing.StartSpan(ctx, "load_multiqc")
	stars, err := multiqc.LoadStarStats(*multiqcPath)
	span.End()
	if err != nil {
		tracing.RecordError(ctx, err)
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("\nparsed STAR stats for %d samples from %s\n", len(stars), *multiqcPath)

	// Trace: classify
	ctx, span = tracing.StartSpan(ctx, "classify")
	findings := classify.UnmappedTooShortOutliers(stars)
	span.End()
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

	// Trace: narrate
	fmt.Println("\nnarrator claims:")
	ctx, span = tracing.StartSpan(ctx, "narrate")
	claims, err := narrator.Narrate(ctx, findings, string(reasoningDoc))
	span.End()
	if err != nil {
		tracing.RecordError(ctx, err)
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
