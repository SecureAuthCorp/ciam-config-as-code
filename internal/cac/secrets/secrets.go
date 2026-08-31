package secrets

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Secret is the local, on-disk representation of a workspace secret.
// Value is only populated by rendered reads (push); it must never be logged.
type Secret struct {
	ID    string `json:"id" yaml:"id"`
	Value string `json:"value" yaml:"value"`
}

// Plan describes what a push would do. Delete holds remote-only IDs and is
// only populated when pruning.
type Plan struct {
	Create []Secret
	Update []Secret
	Delete []string
}

func ComputePlan(local []Secret, remoteIDs []string, prune bool) Plan {
	var (
		plan   Plan
		remote = map[string]bool{}
		seen   = map[string]bool{}
	)

	for _, id := range remoteIDs {
		remote[id] = true
	}

	sorted := slices.Clone(local)
	slices.SortFunc(sorted, func(a, b Secret) int { return strings.Compare(a.ID, b.ID) })

	for _, s := range sorted {
		seen[s.ID] = true

		if remote[s.ID] {
			plan.Update = append(plan.Update, s)
		} else {
			plan.Create = append(plan.Create, s)
		}
	}

	if prune {
		for _, id := range remoteIDs {
			if !seen[id] {
				plan.Delete = append(plan.Delete, id)
			}
		}

		slices.Sort(plan.Delete)
	}

	return plan
}

func (p Plan) Empty() bool {
	return len(p.Create) == 0 && len(p.Update) == 0 && len(p.Delete) == 0
}

// Summary renders the plan with secret IDs only — never values.
func (p Plan) Summary() string {
	var b strings.Builder

	writeSection := func(action string, ids []string) {
		fmt.Fprintf(&b, "%s (%d):\n", action, len(ids))
		for _, id := range ids {
			fmt.Fprintf(&b, "  - %s\n", id)
		}
	}

	writeSection("create", ids(p.Create))
	writeSection("update", ids(p.Update))
	writeSection("delete", p.Delete)

	return b.String()
}

func ids(ss []Secret) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, s.ID)
	}
	return out
}

var nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// NormalizeFileName converts a secret ID into a safe file name (without extension).
func NormalizeFileName(id string) string {
	return nonAlnum.ReplaceAllString(id, "_")
}

var nonEnv = regexp.MustCompile(`[^A-Z0-9]+`)

// EnvVarName derives the conventional environment variable holding a secret's value.
func EnvVarName(id string) string {
	return "CAC_SECRET_" + nonEnv.ReplaceAllString(strings.ToUpper(id), "_")
}
