// Command flowcast parses a real Nextflow execution trace and MultiQC
// data.json, and runs FlowCast's rule-based classifier against them.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"flowcast/internal/classify"
	"flowcast/internal/multiqc"
	"flowcast/internal/nftrace"
)

func main() {
	tracePath := flag.String("trace", "", "path to Nextflow execution_trace.txt")
	multiqcPath := flag.String("multiqc", "", "path to MultiQC multiqc_data.json")
	flag.Parse()

	if *tracePath == "" || *multiqcPath == "" {
		fmt.Fprintln(os.Stderr, "usage: flowcast -trace <execution_trace.txt> -multiqc <multiqc_data.json>")
		os.Exit(2)
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
	}
}
