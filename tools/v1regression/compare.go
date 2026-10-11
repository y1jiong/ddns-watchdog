package main

import (
	"fmt"
	"sort"
	"strings"
)

// Observation is everything a scenario observes about one revision.
type Observation struct {
	ExitCode  int
	Stdout    string
	Stderr    string
	ServerLog string
	Files     map[string]string
	Requests  []string
	Responses []string
}

// Result is the comparison of one scenario between the two revisions.
type Result struct {
	Scenario    Scenario
	V1          *Observation
	V2          *Observation
	Differences []Difference // differences explained by a documented rule
	Unexplained []Difference // differences nothing explains: treated as regressions
	Err         error
}

// Difference is a single observed behavioral difference.
type Difference struct {
	Scenario Scenario
	Field    string
	V1       string
	V2       string
	Why      string
	Expected bool
}

// Intended describes a difference that is deliberate: v2 improved on v1 here.
// Matching is deliberately simple - a rule matches a differing pair when the v1
// side contains V1Match (or V1Match is empty) and the v2 side contains V2Match.
type Intended struct {
	Scenario string // "" matches every scenario
	Field    string // "" matches every field
	V1Match  string
	V2Match  string
	Why      string
}

// runScenario executes one scenario against both revisions and compares.
func runScenario(sc Scenario, targets []*Target) Result {
	res := Result{Scenario: sc}

	obs := make([]*Observation, 0, len(targets))
	for _, target := range targets {
		t, err := target.scoped(sc.String())
		if err != nil {
			res.Err = err
			return res
		}
		defer t.stopProcs()

		o, err := sc.Run(t)
		if err != nil {
			res.Err = fmt.Errorf("%s: %w", t.Ref, err)
			res.V1 = firstOrNil(obs, 0)
			res.V2 = firstOrNil(obs, 1)
			return res
		}
		t.normalizeObservation(o)
		obs = append(obs, o)
	}

	res.V1, res.V2 = obs[0], obs[1]
	for _, d := range compare(sc, obs[0], obs[1]) {
		// a scenario that covers a capability v1 never had: every difference in it
		// is the feature itself
		if sc.NewInV2 {
			d.Expected = true
			d.Why = sc.Why
		}
		if d.Expected {
			res.Differences = append(res.Differences, d)
		} else {
			res.Unexplained = append(res.Unexplained, d)
		}
	}
	return res
}

func firstOrNil(obs []*Observation, i int) *Observation {
	if i < len(obs) {
		return obs[i]
	}
	return &Observation{}
}

type field struct {
	name    string
	v1, v2  []string
	ordered bool
}

func fieldsOf(o *Observation) []field {
	files := make([]string, 0, len(o.Files))
	for k := range o.Files {
		files = append(files, k)
	}
	sort.Strings(files)

	fs := []field{
		{name: "exit_code", v1: []string{fmt.Sprint(o.ExitCode)}, ordered: true},
		{name: "stdout", ordered: false},
		{name: "stderr", ordered: false},
		{name: "server_log", ordered: false},
		{name: "requests", v1: o.Requests, ordered: true},
		{name: "responses", v1: o.Responses, ordered: true},
	}
	fs[1].v1 = splitLines(o.Stdout)
	fs[2].v1 = splitLines(o.Stderr)
	fs[3].v1 = splitLines(o.ServerLog)
	for _, f := range files {
		fs = append(fs, field{name: "file:" + f, v1: splitLines(o.Files[f]), ordered: true})
	}
	return fs
}

func splitLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimRight(line, " \t\r")
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// compare produces the difference list for every compared field.
func compare(sc Scenario, v1, v2 *Observation) []Difference {
	var out []Difference

	f1 := fieldsOf(v1)
	f2 := fieldsOf(v2)

	byName2 := map[string]field{}
	for _, f := range f2 {
		byName2[f.name] = f
	}

	seen := map[string]bool{}
	for _, a := range f1 {
		seen[a.name] = true
		b, ok := byName2[a.name]
		if !ok {
			out = append(out, Difference{Scenario: sc, Field: a.name,
				V1: strings.Join(a.v1, "\n"), V2: "<missing in v2>"})
			continue
		}
		out = append(out, compareField(sc, a.name, a, b)...)
	}
	for _, b := range f2 {
		if !seen[b.name] {
			out = append(out, Difference{Scenario: sc, Field: b.name,
				V1: "<missing in v1>", V2: strings.Join(b.v1, "\n")})
		}
	}

	for i := range out {
		classify(&out[i])
	}
	return out
}

// compareField diffs one field. When the two sides contain the same lines but in
// a different order, the order difference is reported separately, because for
// request sequences the order is part of the behavior.
func compareField(sc Scenario, name string, a, b field) []Difference {
	if equalLines(a.v1, b.v1) {
		return nil
	}

	var out []Difference
	onlyV1 := multisetSubtract(a.v1, b.v1)
	onlyV2 := multisetSubtract(b.v1, a.v1)

	if a.ordered && len(onlyV1) == 0 && len(onlyV2) == 0 {
		return []Difference{{
			Scenario: sc, Field: name + " (order)",
			V1: strings.Join(a.v1, "\n"), V2: strings.Join(b.v1, "\n"),
		}}
	}

	for _, pair := range pairUp(onlyV1, onlyV2) {
		out = append(out, Difference{Scenario: sc, Field: name, V1: pair[0], V2: pair[1]})
	}
	return out
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// multisetSubtract returns the lines of a that are not covered by b.
func multisetSubtract(a, b []string) []string {
	remaining := map[string]int{}
	for _, line := range b {
		remaining[line]++
	}
	var out []string
	for _, line := range a {
		if remaining[line] > 0 {
			remaining[line]--
			continue
		}
		out = append(out, line)
	}
	return out
}

// pairUp lines up the lines that exist on only one side, so a rule can describe
// "v1 said X, v2 says Y".
func pairUp(onlyV1, onlyV2 []string) [][2]string {
	n := len(onlyV1)
	if len(onlyV2) > n {
		n = len(onlyV2)
	}
	var out [][2]string
	for i := 0; i < n; i++ {
		var l, r string
		if i < len(onlyV1) {
			l = onlyV1[i]
		}
		if i < len(onlyV2) {
			r = onlyV2[i]
		}
		out = append(out, [2]string{l, r})
	}
	return out
}

func classify(d *Difference) {
	for _, rule := range intended {
		if rule.Scenario != "" && rule.Scenario != d.Scenario.Name {
			continue
		}
		if rule.Field != "" && rule.Field != d.Field {
			continue
		}
		if rule.V1Match != "" && !strings.Contains(d.V1, rule.V1Match) {
			continue
		}
		if rule.V2Match != "" && !strings.Contains(d.V2, rule.V2Match) {
			continue
		}
		if rule.V1Match == "" && rule.V2Match == "" {
			continue
		}
		d.Expected = true
		d.Why = rule.Why
		return
	}
}
