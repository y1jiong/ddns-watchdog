// Command v1regression is a differential regression harness: it builds two
// revisions of ddns-watchdog (v1 and v2 by default), drives both through the same
// scenarios against local fake services, and diffs what they do.
//
// It answers the v2 requirement "v2 must be a faithful regression of v1 unless v1
// had a bug". Every difference is reported together with a reason; a difference
// that no documented rule explains is treated as a regression and fails the run.
//
// Usage:
//
//	go run ./tools/v1regression                 # v1 vs HEAD, report to stdout
//	go run ./tools/v1regression -ref1 v1.5.12   # compare another baseline
//	go run ./tools/v1regression -ref2 worktree  # include uncommitted changes
//	go run ./tools/v1regression -scenario client/* -v
//
// The harness needs a git checkout (it extracts the trees with "git archive") and
// a Go toolchain. It never touches the working tree.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var (
		ref1     = flag.String("ref1", "v1", "baseline git revision")
		ref2     = flag.String("ref2", "HEAD", "revision under test")
		work     = flag.String("work", "", "working directory (default: a temp dir)")
		scenario = flag.String("scenario", "*", "scenario name filter, e.g. 'client/*'")
		verbose  = flag.Bool("v", false, "print each scenario's observation summary")
		report   = flag.String("report", "", "write a markdown report to this path")
		keep     = flag.Bool("keep", false, "keep the working directory")
	)
	flag.Parse()

	root := *work
	if root == "" {
		dir, err := os.MkdirTemp("", "v1regression-")
		must(err)
		root = dir
	}
	must(os.MkdirAll(root, 0o755))
	if !*keep && *work == "" {
		defer os.RemoveAll(root)
	}
	fmt.Printf("work dir: %s\n", root)

	targets := make([]*Target, 0, 2)
	for _, ref := range []string{*ref1, *ref2} {
		t, err := buildTarget(root, ref)
		must(err)
		fmt.Printf("built %-8s version %-8s -> %s\n", ref, t.Version, t.Bin["client"])
		targets = append(targets, t)
	}

	scenarios := filterScenarios(*scenario)
	if len(scenarios) == 0 {
		fmt.Fprintln(os.Stderr, "no scenario matches", *scenario)
		os.Exit(2)
	}

	var results []Result
	for _, sc := range scenarios {
		res := runScenario(sc, targets)
		results = append(results, res)
		printScenarioResult(res, *verbose)
	}

	report1 := renderReport(*ref1, *ref2, targets, results)
	if *report != "" {
		must(os.WriteFile(*report, []byte(report1), 0o644))
		fmt.Printf("\nreport written to %s\n", *report)
	}
	fmt.Print("\n" + summary(results))

	if unexplained(results) > 0 {
		os.Exit(1)
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "harness error:", err)
		os.Exit(2)
	}
}

func filterScenarios(pattern string) []Scenario {
	var out []Scenario
	for _, sc := range allScenarios {
		if ok, _ := filepath.Match(pattern, sc.Group+"/"+sc.Name); ok || pattern == "*" || pattern == sc.Group+"/*" {
			out = append(out, sc)
		}
	}
	return out
}

func printScenarioResult(res Result, verbose bool) {
	status := "SAME"
	switch {
	case len(res.Unexplained) > 0:
		status = fmt.Sprintf("REGRESSION (%d)", len(res.Unexplained))
	case len(res.Differences) > 0:
		status = fmt.Sprintf("ok (%d intended)", len(res.Differences))
	}
	fmt.Printf("  %-6s %s/%s\n", status, res.Scenario.Group, res.Scenario.Name)

	if verbose {
		for _, d := range res.Differences {
			fmt.Printf("           intended %s: %s\n", d.Field, oneLine(d.Why))
			fmt.Printf("             v1: %s\n", oneLine(d.V1))
			fmt.Printf("             v2: %s\n", oneLine(d.V2))
		}
	}
	for _, d := range res.Unexplained {
		fmt.Printf("           UNEXPLAINED %s/%s\n", d.Scenario.Group, d.Scenario.Name)
		fmt.Printf("             field %s\n", d.Field)
		fmt.Printf("             v1: %s\n", oneLine(d.V1))
		fmt.Printf("             v2: %s\n", oneLine(d.V2))
	}
	if res.Err != nil {
		fmt.Printf("           harness error: %v\n", res.Err)
	}
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ⏎ ")
	if len(s) > 220 {
		s = s[:220] + "…"
	}
	return s
}

func summary(results []Result) string {
	var same, intended, broken, errored int
	for _, r := range results {
		switch {
		case r.Err != nil:
			errored++
		case len(r.Unexplained) > 0:
			broken++
		case len(r.Differences) > 0:
			intended++
		default:
			same++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "scenarios: %d total | %d identical | %d intended differences | %d regressions | %d harness errors\n",
		len(results), same, intended, broken, errored)
	if broken == 0 && errored == 0 {
		b.WriteString("no unexplained differences\n")
	}
	return b.String()
}

func unexplained(results []Result) int {
	n := 0
	for _, r := range results {
		n += len(r.Unexplained)
		if r.Err != nil {
			n++
		}
	}
	return n
}
