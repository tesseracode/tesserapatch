//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

type s7ObservedRegistrationTarget struct {
	row    string
	target s7FixtureTarget
}

type s7ObservedMarkerExpectation struct {
	key          string
	path         string
	token        string
	expectedTest string
}

type s7ObservedMarkerPlan struct {
	overlayPath  string
	backingFiles []string
	expectations map[string]s7ObservedMarkerExpectation
}

type s7ObservedOverlayMutation func(*s7ObservedMarkerPlan) error

type s7GoTestEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
}

func TestS7ObservedAMThroughAORegistrationAuthority(t *testing.T) {
	targets := s7ObservedAMThroughAOTargets(t)
	if len(targets) != 54 {
		t.Fatalf("observed AM-AO row targets = %d, want 54", len(targets))
	}
	required := append([]s7ObservedRegistrationTarget(nil), targets...)
	for _, target := range s7PIB443ObservedLeaves(t) {
		key := s7ObservedTargetKey(target.target)
		alreadyRequired := false
		for _, existing := range required {
			if s7ObservedTargetKey(existing.target) == key {
				alreadyRequired = true
				break
			}
		}
		if !alreadyRequired {
			required = append(required, target)
		}
	}
	if len(required) != 65 {
		t.Fatalf("observed AM-AO unique leaf events = %d, want 65", len(required))
	}

	runS7ObservedCategory(
		t, s7ObservedCategoryAMThroughAO, required,
	)
}

type s7ObservedCategory string

const (
	s7ObservedCategoryAMThroughAO s7ObservedCategory = "AM-AO"
	s7ObservedCategoryAP          s7ObservedCategory = "AP"
	s7ObservedCategoryAQ          s7ObservedCategory = "AQ"
	s7ObservedCategoryAR          s7ObservedCategory = "AR"
	s7ObservedCategoryAS          s7ObservedCategory = "AS"
	s7ObservedCategoryAT          s7ObservedCategory = "AT"
	s7ObservedCategoryAU          s7ObservedCategory = "AU"
	s7ObservedCategoryAV          s7ObservedCategory = "AV"
	s7ObservedCIPackageLimit                         = 40 * time.Minute
)

type s7ObservedHostedBudget struct {
	outer   time.Duration
	inner   time.Duration
	cleanup time.Duration
	first   int
	last    int
	targets int
}

var s7ObservedHostedBudgets = map[s7ObservedCategory]s7ObservedHostedBudget{
	s7ObservedCategoryAMThroughAO: {
		outer: 8 * time.Minute, inner: 4 * time.Minute, cleanup: time.Minute,
		first: 395, last: 448, targets: 65,
	},
	s7ObservedCategoryAP: {
		outer: 12 * time.Minute, inner: 8 * time.Minute, cleanup: time.Minute,
		first: 449, last: 482, targets: 34,
	},
	s7ObservedCategoryAQ: {
		outer: 12 * time.Minute, inner: 8 * time.Minute, cleanup: time.Minute,
		first: 483, last: 505, targets: 23,
	},
	s7ObservedCategoryAR: {
		outer: 39 * time.Minute, inner: 37 * time.Minute, cleanup: time.Minute,
		first: 506, last: 520, targets: 15,
	},
	s7ObservedCategoryAS: {
		outer: 8 * time.Minute, inner: 4 * time.Minute, cleanup: time.Minute,
		first: 521, last: 530, targets: 10,
	},
	s7ObservedCategoryAT: {
		outer: 8 * time.Minute, inner: 4 * time.Minute, cleanup: time.Minute,
		first: 531, last: 536, targets: 6,
	},
	s7ObservedCategoryAU: {
		outer: 8 * time.Minute, inner: 4 * time.Minute, cleanup: time.Minute,
		first: 537, last: 545, targets: 9,
	},
	s7ObservedCategoryAV: {
		outer: 8 * time.Minute, inner: 4 * time.Minute, cleanup: time.Minute,
		first: 546, last: 551, targets: 6,
	},
}

func cloneS7ObservedHostedBudgets(
	source map[s7ObservedCategory]s7ObservedHostedBudget,
) map[s7ObservedCategory]s7ObservedHostedBudget {
	cloned := make(map[s7ObservedCategory]s7ObservedHostedBudget, len(source))
	for category, budget := range source {
		cloned[category] = budget
	}
	return cloned
}

func validateS7ObservedHostedBudgets(
	budgets map[s7ObservedCategory]s7ObservedHostedBudget,
	packageLimit time.Duration,
) error {
	if packageLimit != 2400*time.Second {
		return fmt.Errorf("hosted CI package budget = %s, want finite 40m", packageLimit)
	}
	want := map[s7ObservedCategory]s7ObservedHostedBudget{
		s7ObservedCategoryAMThroughAO: {
			outer: 480 * time.Second, inner: 240 * time.Second, cleanup: 60 * time.Second,
			first: 395, last: 448, targets: 65,
		},
		s7ObservedCategoryAP: {
			outer: 720 * time.Second, inner: 480 * time.Second, cleanup: 60 * time.Second,
			first: 449, last: 482, targets: 34,
		},
		s7ObservedCategoryAQ: {
			outer: 720 * time.Second, inner: 480 * time.Second, cleanup: 60 * time.Second,
			first: 483, last: 505, targets: 23,
		},
		s7ObservedCategoryAR: {
			outer: 2340 * time.Second, inner: 2220 * time.Second, cleanup: 60 * time.Second,
			first: 506, last: 520, targets: 15,
		},
		s7ObservedCategoryAS: {
			outer: 480 * time.Second, inner: 240 * time.Second, cleanup: 60 * time.Second,
			first: 521, last: 530, targets: 10,
		},
		s7ObservedCategoryAT: {
			outer: 480 * time.Second, inner: 240 * time.Second, cleanup: 60 * time.Second,
			first: 531, last: 536, targets: 6,
		},
		s7ObservedCategoryAU: {
			outer: 480 * time.Second, inner: 240 * time.Second, cleanup: 60 * time.Second,
			first: 537, last: 545, targets: 9,
		},
		s7ObservedCategoryAV: {
			outer: 480 * time.Second, inner: 240 * time.Second, cleanup: 60 * time.Second,
			first: 546, last: 551, targets: 6,
		},
	}
	if len(budgets) != len(want) {
		return fmt.Errorf("hosted observer budget count = %d, want %d", len(budgets), len(want))
	}
	for category, budget := range budgets {
		if budget.outer <= 0 || budget.inner <= 0 || budget.cleanup <= 0 {
			return fmt.Errorf("%s hosted observer budget is not finite and positive: %+v", category, budget)
		}
		if budget.inner >= budget.outer-budget.cleanup {
			return fmt.Errorf(
				"%s hosted observer inner %s is not below outer %s minus cleanup %s",
				category, budget.inner, budget.outer, budget.cleanup,
			)
		}
		if budget.outer >= packageLimit {
			return fmt.Errorf(
				"%s hosted observer outer %s is not below CI package limit %s",
				category, budget.outer, packageLimit,
			)
		}
		expected, ok := want[category]
		if !ok {
			return fmt.Errorf("unknown hosted observer category %q", category)
		}
		if budget != expected {
			return fmt.Errorf("%s hosted observer budget = %+v, want %+v",
				category, budget, expected)
		}
	}
	return nil
}

func TestS7ObservedAPRegistrationAuthority(t *testing.T) {
	runS7ObservedCategory(t, s7ObservedCategoryAP, s7ObservedAPTargets(t))
}

func TestS7ObservedAQRegistrationAuthority(t *testing.T) {
	runS7ObservedCategory(t, s7ObservedCategoryAQ, s7ObservedAQTargets(t))
}

type s7ObservedARProcessGroup struct {
	name        string
	first, last int
}

var s7ObservedARProcessGroups = []s7ObservedARProcessGroup{
	{name: "core", first: 506, last: 517},
	{name: "purge", first: 518, last: 518},
	{name: "claims", first: 519, last: 520},
}

func TestS7ObservedARCoreRegistrationAuthority(t *testing.T) {
	runS7ObservedARProcessGroup(t, "core")
}

func TestS7ObservedARPurgeRegistrationAuthority(t *testing.T) {
	runS7ObservedARProcessGroup(t, "purge")
}

func TestS7ObservedARClaimsRegistrationAuthority(t *testing.T) {
	runS7ObservedARProcessGroup(t, "claims")
}

func TestS7ObservedASRegistrationAuthority(t *testing.T) {
	runS7ObservedCategory(t, s7ObservedCategoryAS, s7ObservedASTargets(t))
}

func TestS7ObservedATRegistrationAuthority(t *testing.T) {
	runS7ObservedCategory(t, s7ObservedCategoryAT, s7ObservedATTargets(t))
}

func TestS7ObservedAURegistrationAuthority(t *testing.T) {
	runS7ObservedCategory(t, s7ObservedCategoryAU, s7ObservedAUTargets(t))
}

func TestS7ObservedAVRegistrationAuthority(t *testing.T) {
	runS7ObservedCategory(t, s7ObservedCategoryAV, s7ObservedAVTargets(t))
}

func runS7ObservedARProcessGroup(t *testing.T, name string) {
	t.Helper()
	if err := validateS7ObservedHostedBudgets(
		s7ObservedHostedBudgets, s7ObservedCIPackageLimit,
	); err != nil {
		t.Fatal(err)
	}
	groups, err := s7PartitionObservedARTargets(
		s7ObservedARTargets(t), s7ObservedARProcessGroups,
	)
	if err != nil {
		t.Fatal(err)
	}
	group, ok := groups[name]
	if !ok {
		t.Fatalf("unknown AR observer process group %q", name)
	}
	budget := s7ObservedHostedBudgets[s7ObservedCategoryAR]
	ctx, cancel := context.WithTimeout(context.Background(), budget.outer)
	defer cancel()
	if err := validateS7ObservedRegistrationsWithHostedBudget(
		ctx, avpRepoRoot(t), group,
		budget.inner, budget.cleanup,
	); err != nil {
		t.Fatal(err)
	}
}

func s7PartitionObservedARTargets(
	targets []s7ObservedRegistrationTarget,
	processGroups []s7ObservedARProcessGroup,
) (map[string][]s7ObservedRegistrationTarget, error) {
	budget, ok := s7ObservedHostedBudgets[s7ObservedCategoryAR]
	if !ok {
		return nil, errors.New("AR observer budget is missing")
	}
	if err := validateS7ObservedCategoryTargets(
		s7ObservedCategoryAR, budget, targets,
	); err != nil {
		return nil, err
	}
	groups := make(map[string][]s7ObservedRegistrationTarget, len(processGroups))
	owners := make(map[string]string, len(targets))
	for _, target := range targets {
		row, err := strconv.Atoi(strings.TrimPrefix(target.row, "PIB-"))
		if err != nil {
			return nil, fmt.Errorf("AR observer row %q is invalid", target.row)
		}
		owner := ""
		for _, group := range processGroups {
			if row >= group.first && row <= group.last {
				if owner != "" {
					return nil, fmt.Errorf(
						"AR observer row %s belongs to both %s and %s",
						target.row, owner, group.name,
					)
				}
				owner = group.name
			}
		}
		if owner == "" {
			return nil, fmt.Errorf("AR observer row %s has no process group", target.row)
		}
		key := s7ObservedTargetKey(target.target)
		if previous := owners[key]; previous != "" {
			return nil, fmt.Errorf(
				"AR observer target %s belongs to both %s and %s",
				key, previous, owner,
			)
		}
		owners[key] = owner
		groups[owner] = append(groups[owner], target)
	}
	for _, group := range processGroups {
		want := group.last - group.first + 1
		if len(groups[group.name]) != want {
			return nil, fmt.Errorf(
				"AR observer group %s has %d targets, want %d",
				group.name, len(groups[group.name]), want,
			)
		}
	}
	return groups, nil
}

func runS7ObservedCategory(
	t *testing.T,
	category s7ObservedCategory,
	targets []s7ObservedRegistrationTarget,
) {
	t.Helper()
	if err := validateS7ObservedHostedBudgets(
		s7ObservedHostedBudgets, s7ObservedCIPackageLimit,
	); err != nil {
		t.Fatal(err)
	}
	budget, ok := s7ObservedHostedBudgets[category]
	if !ok {
		t.Fatalf("observed category %q has no hosted budget", category)
	}
	if err := validateS7ObservedCategoryTargets(category, budget, targets); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget.outer)
	defer cancel()
	if err := validateS7ObservedRegistrationsWithHostedBudget(
		ctx, avpRepoRoot(t), targets,
		budget.inner, budget.cleanup,
	); err != nil {
		t.Fatal(err)
	}
}

func validateS7ObservedCategoryTargets(
	category s7ObservedCategory,
	budget s7ObservedHostedBudget,
	targets []s7ObservedRegistrationTarget,
) error {
	wantCount := budget.targets
	if wantCount <= 0 || len(targets) != wantCount {
		return fmt.Errorf("%s observed row targets = %d, want %d",
			category, len(targets), wantCount)
	}
	seen := make(map[string]int, len(targets))
	for _, target := range targets {
		seen[target.row]++
	}
	for id := budget.first; id <= budget.last; id++ {
		row := fmt.Sprintf("PIB-%03d", id)
		expected := 1
		if category == s7ObservedCategoryAMThroughAO && row == "PIB-443" {
			expected = 12
		}
		if seen[row] != expected {
			return fmt.Errorf("%s observed targets omit %s", category, row)
		}
	}
	return nil
}

func TestS7ObservedRegistrationWrongInputs(t *testing.T) {
	t.Run("exact-regex-escaping", func(t *testing.T) {
		pattern := s7ObservedTopLevelPattern([]string{
			"TestExact",
			"TestMeta[1].*",
		})
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, exact := range []string{"TestExact", "TestMeta[1].*"} {
			if !compiled.MatchString(exact) {
				t.Fatalf("escaped observed-test regex does not match %q: %s", exact, pattern)
			}
		}
		for _, expanded := range []string{"TestExactExtra", "TestMeta1anything"} {
			if compiled.MatchString(expanded) {
				t.Fatalf("escaped observed-test regex overmatched %q: %s", expanded, pattern)
			}
		}
	})

	t.Run("external-workspace-mode", func(t *testing.T) {
		repoRoot := avpRepoRoot(t)
		workspace, err := s7CreateObservedWorkspace(repoRoot)
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(workspace)
		outside, err := s7ObservedPathOutside(repoRoot, workspace)
		if err != nil || !outside {
			t.Fatalf("workspace outside repository = %t, err=%v", outside, err)
		}
		info, err := os.Stat(workspace)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("workspace mode = %04o, want 0700", info.Mode().Perm())
		}
	})
	t.Run("ar-process-partition", func(t *testing.T) {
		targets := s7ObservedARTargets(t)
		groups, err := s7PartitionObservedARTargets(
			targets, s7ObservedARProcessGroups,
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(groups) != 3 ||
			len(groups["core"]) != 12 ||
			len(groups["purge"]) != 1 ||
			len(groups["claims"]) != 2 {
			t.Fatalf("AR observer process groups = %v", groups)
		}
		for _, fixture := range []struct {
			name    string
			targets []s7ObservedRegistrationTarget
			groups  []s7ObservedARProcessGroup
		}{
			{
				name:    "missing-target",
				targets: targets[:len(targets)-1],
				groups:  s7ObservedARProcessGroups,
			},
			{
				name:    "overlapping-ranges",
				targets: targets,
				groups: []s7ObservedARProcessGroup{
					{name: "core", first: 506, last: 518},
					{name: "purge", first: 518, last: 518},
					{name: "claims", first: 519, last: 520},
				},
			},
			{
				name:    "gap",
				targets: targets,
				groups: []s7ObservedARProcessGroup{
					{name: "core", first: 506, last: 517},
					{name: "purge", first: 519, last: 519},
					{name: "claims", first: 520, last: 520},
				},
			},
		} {
			t.Run(fixture.name, func(t *testing.T) {
				if _, err := s7PartitionObservedARTargets(
					fixture.targets, fixture.groups,
				); err == nil {
					t.Fatal("AR observer partition accepted wrong input")
				}
			})
		}
	})

	t.Run("hosted-budget-order", func(t *testing.T) {
		if err := validateS7ObservedHostedBudgets(
			s7ObservedHostedBudgets, s7ObservedCIPackageLimit,
		); err != nil {
			t.Fatal(err)
		}
		for _, category := range []s7ObservedCategory{
			s7ObservedCategoryAMThroughAO,
			s7ObservedCategoryAP,
			s7ObservedCategoryAQ,
			s7ObservedCategoryAR,
			s7ObservedCategoryAS,
			s7ObservedCategoryAT,
			s7ObservedCategoryAU,
			s7ObservedCategoryAV,
		} {
			budget := s7ObservedHostedBudgets[category]
			t.Run(string(category), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), budget.outer)
				defer cancel()
				got := s7ObservedInnerTimeoutWithCleanup(
					ctx, budget.inner, budget.cleanup,
				)
				if got <= 0 || got > budget.inner || got >= budget.outer ||
					budget.outer-got < budget.cleanup {
					t.Fatalf(
						"%s hosted observed budget = inner:%s outer:%s cleanup:%s",
						category, got, budget.outer, budget.cleanup,
					)
				}
			})
		}
		for _, fixture := range []struct {
			name   string
			mutate func(map[s7ObservedCategory]s7ObservedHostedBudget)
		}{
			{
				name: "accidental-global-eight-minute-replacement",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					ap := budgets[s7ObservedCategoryAP]
					ap.outer = 12 * time.Minute
					budgets[s7ObservedCategoryAP] = ap
					aq := budgets[s7ObservedCategoryAQ]
					aq.inner = 12 * time.Minute
					budgets[s7ObservedCategoryAQ] = aq
				},
			},
			{
				name: "wrong-ap-outer-only",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					ap := budgets[s7ObservedCategoryAP]
					ap.outer = 8 * time.Minute
					budgets[s7ObservedCategoryAP] = ap
				},
			},
			{
				name: "wrong-ap-inner-only",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					ap := budgets[s7ObservedCategoryAP]
					ap.inner = 4 * time.Minute
					budgets[s7ObservedCategoryAP] = ap
				},
			},
			{
				name: "wrong-am-ao-inner-only",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					amAO := budgets[s7ObservedCategoryAMThroughAO]
					amAO.inner = 90 * time.Second
					budgets[s7ObservedCategoryAMThroughAO] = amAO
				},
			},
			{
				name: "category-tuples-swapped",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					ap := budgets[s7ObservedCategoryAP]
					budgets[s7ObservedCategoryAP] = budgets[s7ObservedCategoryAQ]
					budgets[s7ObservedCategoryAQ] = ap
				},
			},
			{
				name: "ar-range-swapped-with-ap",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					ap := budgets[s7ObservedCategoryAP]
					budgets[s7ObservedCategoryAP] = budgets[s7ObservedCategoryAR]
					budgets[s7ObservedCategoryAR] = ap
				},
			},
			{
				name: "wrong-value",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					aq := budgets[s7ObservedCategoryAQ]
					aq.inner = 7 * time.Minute
					budgets[s7ObservedCategoryAQ] = aq
				},
			},
			{
				name: "wrong-ar-range",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					ar := budgets[s7ObservedCategoryAR]
					ar.first = 505
					budgets[s7ObservedCategoryAR] = ar
				},
			},
			{
				name: "wrong-ar-outer-only",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					ar := budgets[s7ObservedCategoryAR]
					ar.outer = 35 * time.Minute
					budgets[s7ObservedCategoryAR] = ar
				},
			},
			{
				name: "wrong-ar-inner-only",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					ar := budgets[s7ObservedCategoryAR]
					ar.inner = 30 * time.Minute
					budgets[s7ObservedCategoryAR] = ar
				},
			},
			{
				name: "wrong-ar-cleanup-only",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					ar := budgets[s7ObservedCategoryAR]
					ar.cleanup = 2 * time.Minute
					budgets[s7ObservedCategoryAR] = ar
				},
			},
			{
				name: "missing-category",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					delete(budgets, s7ObservedCategoryAQ)
				},
			},
			{
				name: "missing-ar-category",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					delete(budgets, s7ObservedCategoryAR)
				},
			},
			{
				name: "wrong-as-range",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					as := budgets[s7ObservedCategoryAS]
					as.last = 531
					budgets[s7ObservedCategoryAS] = as
				},
			},
			{
				name: "missing-as-category",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					delete(budgets, s7ObservedCategoryAS)
				},
			},
			{
				name: "wrong-at-range",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					at := budgets[s7ObservedCategoryAT]
					at.last = 537
					budgets[s7ObservedCategoryAT] = at
				},
			},
			{
				name: "missing-at-category",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					delete(budgets, s7ObservedCategoryAT)
				},
			},
			{
				name: "wrong-au-range",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					au := budgets[s7ObservedCategoryAU]
					au.last = 546
					budgets[s7ObservedCategoryAU] = au
				},
			},
			{
				name: "missing-au-category",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					delete(budgets, s7ObservedCategoryAU)
				},
			},
			{
				name: "wrong-av-range",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					av := budgets[s7ObservedCategoryAV]
					av.last = 552
					budgets[s7ObservedCategoryAV] = av
				},
			},
			{
				name: "wrong-av-target-count",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					av := budgets[s7ObservedCategoryAV]
					av.targets = 5
					budgets[s7ObservedCategoryAV] = av
				},
			},
			{
				name: "missing-av-category",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					delete(budgets, s7ObservedCategoryAV)
				},
			},
			{
				name: "extra-category",
				mutate: func(budgets map[s7ObservedCategory]s7ObservedHostedBudget) {
					budgets["AY"] = s7ObservedHostedBudget{
						outer: 8 * time.Minute, inner: 4 * time.Minute, cleanup: time.Minute,
						first: 568, last: 568, targets: 1,
					}
				},
			},
		} {
			t.Run(fixture.name, func(t *testing.T) {
				budgets := cloneS7ObservedHostedBudgets(s7ObservedHostedBudgets)
				fixture.mutate(budgets)
				if err := validateS7ObservedHostedBudgets(
					budgets, s7ObservedCIPackageLimit,
				); err == nil {
					t.Fatal("hosted observed budget guard accepted wrong category budgets")
				}
			})
		}
		t.Run("callsite-category-binding", func(t *testing.T) {
			if err := validateS7ObservedCategoryTargets(
				s7ObservedCategoryAMThroughAO,
				s7ObservedHostedBudgets[s7ObservedCategoryAMThroughAO],
				s7ObservedAPTargets(t),
			); err == nil {
				t.Fatal("AM-AO category key accepted AP observer targets")
			}
			if err := validateS7ObservedCategoryTargets(
				s7ObservedCategoryAP,
				s7ObservedHostedBudgets[s7ObservedCategoryAP],
				s7ObservedAQTargets(t),
			); err == nil {
				t.Fatal("AP category key accepted AQ observer targets")
			}
			if err := validateS7ObservedCategoryTargets(
				s7ObservedCategoryAQ,
				s7ObservedHostedBudgets[s7ObservedCategoryAQ],
				s7ObservedAPTargets(t),
			); err == nil {
				t.Fatal("AQ category key accepted AP observer targets")
			}
			if err := validateS7ObservedCategoryTargets(
				s7ObservedCategoryAR,
				s7ObservedHostedBudgets[s7ObservedCategoryAR],
				s7ObservedAQTargets(t),
			); err == nil {
				t.Fatal("AR category key accepted AQ observer targets")
			}
			if err := validateS7ObservedCategoryTargets(
				s7ObservedCategoryAS,
				s7ObservedHostedBudgets[s7ObservedCategoryAS],
				s7ObservedARTargets(t),
			); err == nil {
				t.Fatal("AS category key accepted AR observer targets")
			}
			if err := validateS7ObservedCategoryTargets(
				s7ObservedCategoryAT,
				s7ObservedHostedBudgets[s7ObservedCategoryAT],
				s7ObservedASTargets(t),
			); err == nil {
				t.Fatal("AT category key accepted AS observer targets")
			}
			if err := validateS7ObservedCategoryTargets(
				s7ObservedCategoryAU,
				s7ObservedHostedBudgets[s7ObservedCategoryAU],
				s7ObservedATTargets(t),
			); err == nil {
				t.Fatal("AU category key accepted AT observer targets")
			}
			if err := validateS7ObservedCategoryTargets(
				s7ObservedCategoryAV,
				s7ObservedHostedBudgets[s7ObservedCategoryAV],
				s7ObservedAUTargets(t),
			); err == nil {
				t.Fatal("AV category key accepted AU observer targets")
			}
		})
	})

	fixtures := []struct {
		name    string
		test    string
		timeout time.Duration
	}{
		{
			name:    "baseline-registers-exactly",
			test:    "TestS7FixtureBaseline",
			timeout: 30 * time.Second,
		},
		{
			name:    "infinite-loop-before-target",
			test:    "TestS7FixtureInfinite",
			timeout: 3 * time.Second,
		},
		{
			name:    "short-circuit-target",
			test:    "TestS7FixtureShortCircuit",
			timeout: 30 * time.Second,
		},
		{
			name:    "aliased-skip-terminator",
			test:    "TestS7FixtureAliasedSkip",
			timeout: 30 * time.Second,
		},
		{
			name:    "nested-target-not-invoked",
			test:    "TestS7FixtureNested",
			timeout: 30 * time.Second,
		},
		{
			name:    "framed-output-and-old-marker-forgery",
			test:    "TestS7FixtureFramedForgery",
			timeout: 30 * time.Second,
		},
	}
	const packageDirectory = "internal/cli/testdata/s7registration"
	packagePath := s7ObservedPackagePath(packageDirectory)
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			expected := []s7ObservedRegistrationTarget{{
				row: "fixture",
				target: s7FixtureTarget{
					dir: packageDirectory, pkg: "s7registration",
					test: fixture.test, subtest: "target",
				},
			}}
			ctx, cancel := context.WithTimeout(context.Background(), fixture.timeout)
			defer cancel()
			started := time.Now()
			err := validateS7ObservedRegistrations(
				ctx,
				avpRepoRoot(t),
				expected,
			)
			if fixture.name == "baseline-registers-exactly" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil ||
				!strings.Contains(err.Error(), packagePath+"/"+fixture.test+"/target") {
				t.Fatalf("same observed-registration validator accepted %s: %v", fixture.name, err)
			}
			if fixture.name == "infinite-loop-before-target" {
				if elapsed := time.Since(started); elapsed >= 8*time.Second {
					t.Fatalf("infinite-loop sensitivity exceeded reap bound: %s", elapsed)
				}
				if !strings.Contains(err.Error(), "timed out") ||
					strings.Contains(err.Error(), "still alive") {
					t.Fatalf("infinite-loop sensitivity did not prove bounded reap: %v", err)
				}
			}
			if fixture.name == "framed-output-and-old-marker-forgery" &&
				!strings.Contains(err.Error(), "JSON diagnostics: <none>") {
				t.Fatalf("framed output did not forge diagnostic RUN/PASS events: %v", err)
			}
		})
	}

	t.Run("correlation-token-and-association", func(t *testing.T) {
		expected := []s7ObservedRegistrationTarget{
			{
				row: "first",
				target: s7FixtureTarget{
					dir: packageDirectory, pkg: "s7registration",
					test: "TestS7FixtureCorrelation", subtest: "first",
				},
			},
			{
				row: "second",
				target: s7FixtureTarget{
					dir: packageDirectory, pkg: "s7registration",
					test: "TestS7FixtureCorrelation", subtest: "second",
				},
			},
		}
		mutations := []struct {
			name   string
			mutate func(*s7ObservedMarkerPlan) error
		}{
			{
				name: "target-association",
				mutate: func(plan *s7ObservedMarkerPlan) error {
					firstKey := s7ObservedTargetKey(expected[0].target)
					secondKey := s7ObservedTargetKey(expected[1].target)
					first := plan.expectations[firstKey]
					second := plan.expectations[secondKey]
					first.path, second.path = second.path, first.path
					plan.expectations[firstKey] = first
					plan.expectations[secondKey] = second
					return nil
				},
			},
			{
				name: "correlation-token",
				mutate: func(plan *s7ObservedMarkerPlan) error {
					key := s7ObservedTargetKey(expected[0].target)
					expectation := plan.expectations[key]
					expectation.token += "-wrong"
					plan.expectations[key] = expectation
					return nil
				},
			},
		}
		for _, mutation := range mutations {
			t.Run(mutation.name, func(t *testing.T) {
				var scratchPaths []string
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				err := validateS7ObservedRegistrationsWithMutation(
					ctx,
					avpRepoRoot(t),
					expected,
					func(plan *s7ObservedMarkerPlan) error {
						scratchPaths = append(
							scratchPaths,
							filepath.Dir(plan.overlayPath),
						)
						scratchPaths = append(scratchPaths, plan.backingFiles...)
						return mutation.mutate(plan)
					},
				)
				if err == nil || !strings.Contains(
					err.Error(),
					packagePath+"/TestS7FixtureCorrelation/",
				) {
					t.Fatalf("same validator accepted %s mutation: %v", mutation.name, err)
				}
				for _, path := range scratchPaths {
					if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
						t.Fatalf("validator retained correlation scratch %s: %v", path, statErr)
					}
				}
			})
		}
	})
}

func s7ObservedAMThroughAOTargets(t *testing.T) []s7ObservedRegistrationTarget {
	t.Helper()
	rows := append([]s7AMLedgerRow(nil), s7AMCoverageLedger()...)
	rows = append(rows, s7ANCoverageLedger()...)
	rows = append(rows, s7AOCoverageLedger()...)
	overrides := map[string]s7FixtureTarget{
		"PIB-416": {
			dir: "internal/cli", pkg: "cli",
			test: "TestS7PIB416DeniedFilesystemClassesReachPublicCLIReports",
		},
		"PIB-417": {
			dir: "internal/cli", pkg: "cli",
			test:    "TestS7BSDPlatformPredicateSeamRuntime",
			subtest: "PIB-417/check-ready-human.txt",
		},
	}
	if runtime.GOOS == "darwin" {
		overrides["PIB-441"] = s7FixtureTarget{
			dir: "internal/intentlock", pkg: "intentlock",
			test: "TestS7PIB441DarwinRootFilesystemPolicyFixtures",
		}
	} else {
		overrides["PIB-441"] = s7FixtureTarget{
			dir: "internal/intentlock", pkg: "intentlock",
			test: "TestS7PIB441LinuxRootFilesystemPolicyFixtures",
		}
	}

	observed := make([]s7ObservedRegistrationTarget, 0, len(rows))
	seen := map[string]string{}
	for _, row := range rows {
		selected, overridden := overrides[row.id]
		if !overridden {
			if len(row.targets) == 0 {
				t.Fatalf("%s has no observed-registration candidate", row.id)
			}
			selected = row.targets[0]
		}
		key := s7ObservedTargetKey(selected)
		if previous := seen[key]; previous != "" {
			t.Fatalf("%s and %s share observed-registration target %s", previous, row.id, key)
		}
		seen[key] = row.id
		observed = append(observed, s7ObservedRegistrationTarget{
			row: row.id, target: selected,
		})
	}
	return observed
}

func s7ObservedAPTargets(t *testing.T) []s7ObservedRegistrationTarget {
	t.Helper()
	rows := s7APCoverageLedger(t)
	observed := make([]s7ObservedRegistrationTarget, 0, len(rows))
	seen := map[string]string{}
	for _, row := range rows {
		if len(row.targets) == 0 {
			t.Fatalf("%s has no observed-registration candidate", row.id)
		}
		selected := row.targets[0]
		key := s7ObservedTargetKey(selected)
		if previous := seen[key]; previous != "" {
			t.Fatalf("%s and %s share observed-registration target %s", previous, row.id, key)
		}
		seen[key] = row.id
		observed = append(observed, s7ObservedRegistrationTarget{row: row.id, target: selected})
	}
	return observed
}

func s7ObservedAQTargets(t *testing.T) []s7ObservedRegistrationTarget {
	t.Helper()
	rows := s7AQCoverageLedger(t)
	observed := make([]s7ObservedRegistrationTarget, 0, len(rows))
	seen := map[string]string{}
	for _, row := range rows {
		if len(row.targets) == 0 {
			t.Fatalf("%s has no observed-registration candidate", row.id)
		}
		selected := row.targets[0]
		key := s7ObservedTargetKey(selected)
		if previous := seen[key]; previous != "" {
			t.Fatalf("%s and %s share observed-registration target %s", previous, row.id, key)
		}
		seen[key] = row.id
		observed = append(observed, s7ObservedRegistrationTarget{row: row.id, target: selected})
	}
	return observed
}

func s7ObservedARTargets(t *testing.T) []s7ObservedRegistrationTarget {
	t.Helper()
	rows := s7ARCoverageLedger(t)
	observed := make([]s7ObservedRegistrationTarget, 0, len(rows))
	seen := map[string]string{}
	for _, row := range rows {
		if len(row.targets) == 0 {
			t.Fatalf("%s has no observed-registration candidate", row.id)
		}
		selected := row.targets[0]
		key := s7ObservedTargetKey(selected)
		if previous := seen[key]; previous != "" {
			t.Fatalf("%s and %s share observed-registration target %s", previous, row.id, key)
		}
		seen[key] = row.id
		observed = append(observed, s7ObservedRegistrationTarget{row: row.id, target: selected})
	}
	return observed
}

func s7PIB443ObservedLeaves(t *testing.T) []s7ObservedRegistrationTarget {
	t.Helper()
	for _, row := range s7AOCoverageLedger() {
		if row.id != "PIB-443" {
			continue
		}
		if len(row.targets) != 12 {
			t.Fatalf("PIB-443 observed leaves = %d, want 12", len(row.targets))
		}
		result := make([]s7ObservedRegistrationTarget, 0, len(row.targets))
		for _, target := range row.targets {
			result = append(result, s7ObservedRegistrationTarget{
				row: "PIB-443", target: target,
			})
		}
		return result
	}
	t.Fatal("PIB-443 is missing from the AO ledger")
	return nil
}

func validateS7ObservedRegistrations(
	ctx context.Context,
	repoRoot string,
	expected []s7ObservedRegistrationTarget,
) error {
	return validateS7ObservedRegistrationsWithInnerLimit(
		ctx, repoRoot, expected, 90*time.Second,
	)
}

func validateS7ObservedRegistrationsWithInnerLimit(
	ctx context.Context,
	repoRoot string,
	expected []s7ObservedRegistrationTarget,
	innerLimit time.Duration,
) error {
	return validateS7ObservedRegistrationsWithMutation(
		ctx, repoRoot, expected, nil, innerLimit,
	)
}

func validateS7ObservedRegistrationsWithHostedBudget(
	ctx context.Context,
	repoRoot string,
	expected []s7ObservedRegistrationTarget,
	innerLimit time.Duration,
	cleanupMargin time.Duration,
) error {
	if innerLimit <= 0 || cleanupMargin <= 0 {
		return fmt.Errorf(
			"observed hosted budgets must be finite and positive: inner=%s cleanup=%s",
			innerLimit, cleanupMargin,
		)
	}
	return validateS7ObservedRegistrationsWithMutation(
		ctx, repoRoot, expected, nil, innerLimit, cleanupMargin,
	)
}

func validateS7ObservedRegistrationsWithMutation(
	ctx context.Context,
	repoRoot string,
	expected []s7ObservedRegistrationTarget,
	mutate s7ObservedOverlayMutation,
	innerLimit ...time.Duration,
) error {
	// Per-run markers correlate execution with exact leaf bodies for semantic
	// regression checks. They are not a sandbox: source under validation is
	// assumed not to inspect same-UID overlay files or its own executable.
	if len(expected) == 0 {
		return fmt.Errorf("observed-registration target list is empty")
	}
	topLevel := map[string]bool{}
	packageDirectories := map[string]bool{}
	expectedByKey := map[string]s7ObservedRegistrationTarget{}
	for _, item := range expected {
		key := s7ObservedTargetKey(item.target)
		if _, exists := expectedByKey[key]; exists {
			return fmt.Errorf("duplicate observed-registration target %s", key)
		}
		expectedByKey[key] = item
		topLevel[item.target.test] = true
		packageDirectories[item.target.dir] = true
	}
	for forbidden := range topLevel {
		if strings.Contains(forbidden, "CoverageLedger") ||
			(strings.Contains(forbidden, "Observed") &&
				strings.Contains(forbidden, "RegistrationAuthority")) ||
			strings.Contains(forbidden, "ObservedRegistrationWrongInputs") {
			return fmt.Errorf("observed-registration selection recurses into %s", forbidden)
		}
	}
	testNames := s7SortedObservedKeys(topLevel)
	pattern := s7ObservedTopLevelPattern(testNames)
	for _, name := range testNames {
		if matched, _ := regexp.MatchString(pattern, name); !matched {
			return fmt.Errorf("escaped observed-registration regex lost %s", name)
		}
	}
	for _, excluded := range []string{
		"TestS7RowManifestAndAMCoverageLedger",
		"TestS7ANCoverageLedger",
		"TestS7AOCoverageLedger",
		"TestS7APCoverageLedger",
		"TestS7AQCoverageLedger",
		"TestS7ARCoverageLedger",
		"TestS7ObservedAMThroughAORegistrationAuthority",
		"TestS7ObservedAPRegistrationAuthority",
		"TestS7ObservedAQRegistrationAuthority",
		"TestS7ObservedARCoreRegistrationAuthority",
		"TestS7ObservedARPurgeRegistrationAuthority",
		"TestS7ObservedARClaimsRegistrationAuthority",
		"TestS7ObservedASRegistrationAuthority",
		"TestS7ObservedATRegistrationAuthority",
		"TestS7ObservedAURegistrationAuthority",
		"TestS7ObservedAVRegistrationAuthority",
	} {
		if matched, _ := regexp.MatchString(pattern, excluded); matched {
			return fmt.Errorf("observed-registration regex includes %s", excluded)
		}
	}

	workspace, err := s7CreateObservedWorkspace(repoRoot)
	if err != nil {
		return err
	}
	defer os.RemoveAll(workspace)
	processWorkspace, err := s7CreateObservedWorkspace(repoRoot)
	if err != nil {
		return err
	}
	defer os.RemoveAll(processWorkspace)
	markerDirectory := filepath.Join(workspace, "markers")
	if err := os.Mkdir(markerDirectory, 0o700); err != nil {
		return err
	}
	markerPlan, err := s7BuildObservedMarkerOverlay(
		repoRoot,
		workspace,
		markerDirectory,
		expected,
	)
	if err != nil {
		return err
	}
	if mutate != nil {
		if err := mutate(markerPlan); err != nil {
			return err
		}
	}

	packageArgs := make([]string, 0, len(packageDirectories))
	for directory := range packageDirectories {
		packageArgs = append(packageArgs, "./"+filepath.ToSlash(directory))
	}
	sort.Strings(packageArgs)
	limit := 90 * time.Second
	cleanupMargin := time.Duration(0)
	if len(innerLimit) == 1 {
		limit = innerLimit[0]
	} else if len(innerLimit) == 2 {
		limit = innerLimit[0]
		cleanupMargin = innerLimit[1]
	} else if len(innerLimit) > 1 {
		return fmt.Errorf("observed registration received %d inner timeout limits", len(innerLimit))
	}
	innerTimeout := s7ObservedInnerTimeout(ctx, limit)
	if cleanupMargin != 0 {
		innerTimeout = s7ObservedInnerTimeoutWithCleanup(ctx, limit, cleanupMargin)
	}
	args := []string{
		"test", "-json", "-p=1", "-count=1",
		"-timeout=" + innerTimeout.String(),
		"-overlay=" + markerPlan.overlayPath,
		"-run", pattern,
	}
	args = append(args, packageArgs...)
	command := exec.CommandContext(ctx, "go", args...)
	command.Dir = repoRoot
	command.Env = append(
		os.Environ(),
		"TPATCH_S7_PID_PROBE="+filepath.Join(processWorkspace, "pid"),
	)
	for _, entry := range command.Env {
		if strings.Contains(entry, markerDirectory) {
			return fmt.Errorf("child environment exposes correlation directory")
		}
		for _, expectation := range markerPlan.expectations {
			if strings.Contains(entry, expectation.path) ||
				strings.Contains(entry, expectation.token) {
				return fmt.Errorf("child environment exposes correlation token")
			}
		}
	}
	for _, arg := range args {
		for _, expectation := range markerPlan.expectations {
			if strings.Contains(arg, expectation.path) ||
				strings.Contains(arg, expectation.token) {
				return fmt.Errorf("child argument exposes correlation token")
			}
		}
	}
	s7ConfigureObservedProcess(command)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	runErr := command.Run()
	if err := s7AssertObservedProcessReaped(processWorkspace); err != nil {
		return err
	}

	runCounts := map[string]int{}
	passCounts := map[string]int{}
	var unexpected []string
	scanner := bufio.NewScanner(bytes.NewReader(stdout.Bytes()))
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var event s7GoTestEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return fmt.Errorf("decode go test event: %w", err)
		}
		if event.Test == "" {
			continue
		}
		top := strings.SplitN(event.Test, "/", 2)[0]
		if !topLevel[top] {
			unexpected = append(unexpected, event.Package+"/"+event.Test)
			continue
		}
		key := event.Package + "/" + event.Test
		switch event.Action {
		case "run":
			runCounts[key]++
		case "pass":
			passCounts[key]++
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	var failures []string
	for _, item := range expectedByKey {
		eventKey := s7ObservedPackagePath(item.target.dir) + "/" +
			s7ObservedFullTestName(item.target)
		if runCounts[eventKey] != 1 || passCounts[eventKey] != 1 {
			failures = append(failures, fmt.Sprintf(
				"%s (%s) RUN=%d PASS=%d",
				eventKey, item.row, runCounts[eventKey], passCounts[eventKey],
			))
		}
	}
	sort.Strings(failures)
	sort.Strings(unexpected)
	markerFailures := s7ValidateObservedMarkers(
		markerDirectory,
		markerPlan.expectations,
	)
	timedOut := ctx.Err() != nil ||
		bytes.Contains(stdout.Bytes(), []byte("test timed out")) ||
		bytes.Contains(stderr.Bytes(), []byte("test timed out"))
	if len(markerFailures) != 0 {
		timeoutNote := ""
		if timedOut {
			timeoutNote = " (timed out)"
		}
		jsonDiagnostics := strings.Join(failures, ", ")
		if jsonDiagnostics == "" {
			jsonDiagnostics = "<none>"
		}
		return fmt.Errorf(
			"observed registration missing out-of-band markers%s: %s; JSON diagnostics: %s; stderr: %s",
			timeoutNote,
			strings.Join(markerFailures, ", "),
			jsonDiagnostics,
			strings.TrimSpace(stderr.String()),
		)
	}
	if ctx.Err() != nil {
		return fmt.Errorf("observed registration outer deadline exceeded: %w", ctx.Err())
	}
	if len(failures) != 0 {
		return fmt.Errorf(
			"observed registration missing targets: %s; go test error: %v; stderr: %s",
			strings.Join(failures, ", "),
			runErr,
			strings.TrimSpace(stderr.String()),
		)
	}
	if runErr != nil {
		return fmt.Errorf("observed registration go test: %w; stderr: %s", runErr, stderr.String())
	}
	if len(unexpected) != 0 {
		return fmt.Errorf("observed registration ran unrelated tests: %s", strings.Join(unexpected, ", "))
	}
	return nil
}

type s7ObservedPackageAST struct {
	fileSet        *token.FileSet
	files          []*ast.File
	paths          map[*ast.File]string
	functions      map[string][]*ast.FuncDecl
	testingAliases map[string]bool
}

func s7BuildObservedMarkerOverlay(
	repoRoot string,
	workspace string,
	markerDirectory string,
	expected []s7ObservedRegistrationTarget,
) (*s7ObservedMarkerPlan, error) {
	byDirectory := map[string][]s7ObservedRegistrationTarget{}
	for _, item := range expected {
		byDirectory[item.target.dir] = append(byDirectory[item.target.dir], item)
	}
	replace := map[string]string{}
	plan := &s7ObservedMarkerPlan{
		expectations: map[string]s7ObservedMarkerExpectation{},
	}
	for directory, targets := range byDirectory {
		packageName := targets[0].target.pkg
		for _, target := range targets {
			if target.target.pkg != packageName {
				return nil, fmt.Errorf(
					"%s mixes packages %s and %s",
					directory, packageName, target.target.pkg,
				)
			}
		}
		parsed, err := s7ParseObservedPackage(repoRoot, directory, packageName)
		if err != nil {
			return nil, err
		}
		modified := map[*ast.File]bool{}
		for _, item := range targets {
			file, body, receiver, err := s7LocateObservedTargetBody(parsed, item.target)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", item.row, err)
			}
			markerName, err := s7ObservedRandomHex(24)
			if err != nil {
				return nil, err
			}
			correlationToken, err := s7ObservedRandomHex(32)
			if err != nil {
				return nil, err
			}
			markerPath := filepath.Join(markerDirectory, markerName+".mark")
			expectedTest := s7ObservedFullTestName(item.target)
			body.List = append([]ast.Stmt{
				s7ObservedMarkerStatement(
					receiver.Name, expectedTest, markerPath, correlationToken,
				),
			}, body.List...)
			if err := s7AssertObservedLiteralMarker(
				body, receiver.Name, expectedTest, markerPath, correlationToken,
			); err != nil {
				return nil, fmt.Errorf("%s: %w", item.row, err)
			}
			modified[file] = true
			key := s7ObservedTargetKey(item.target)
			plan.expectations[key] = s7ObservedMarkerExpectation{
				key: key, path: markerPath, token: correlationToken,
				expectedTest: expectedTest,
			}
		}
		for file := range modified {
			if err := s7AddObservedMarkerImport(file); err != nil {
				return nil, err
			}
			original := parsed.paths[file]
			sum := sha256.Sum256([]byte(original))
			backing := filepath.Join(
				workspace,
				"overlay-"+hex.EncodeToString(sum[:])+".go",
			)
			var formatted bytes.Buffer
			if err := format.Node(&formatted, parsed.fileSet, file); err != nil {
				return nil, err
			}
			if err := os.WriteFile(backing, formatted.Bytes(), 0o600); err != nil {
				return nil, err
			}
			replace[original] = backing
			plan.backingFiles = append(plan.backingFiles, backing)
		}
	}
	overlay := struct {
		Replace map[string]string `json:"Replace"`
	}{Replace: replace}
	data, err := json.Marshal(overlay)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(workspace, "overlay.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, err
	}
	plan.overlayPath = path
	return plan, nil
}

func s7ParseObservedPackage(
	repoRoot string,
	directory string,
	packageName string,
) (s7ObservedPackageAST, error) {
	absolute := filepath.Join(repoRoot, filepath.FromSlash(directory))
	entries, err := os.ReadDir(absolute)
	if err != nil {
		return s7ObservedPackageAST{}, err
	}
	parsed := s7ObservedPackageAST{
		fileSet:        token.NewFileSet(),
		paths:          map[*ast.File]string{},
		functions:      map[string][]*ast.FuncDecl{},
		testingAliases: map[string]bool{},
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(absolute, entry.Name())
		file, err := parser.ParseFile(parsed.fileSet, path, nil, parser.ParseComments)
		if err != nil {
			return s7ObservedPackageAST{}, err
		}
		parsed.files = append(parsed.files, file)
		parsed.paths[file] = path
		if file.Name.Name != packageName {
			continue
		}
		for _, imported := range file.Imports {
			pathValue, err := strconv.Unquote(imported.Path.Value)
			if err != nil || pathValue != "testing" {
				continue
			}
			alias := "testing"
			if imported.Name != nil {
				alias = imported.Name.Name
			}
			if alias != "_" && alias != "." {
				parsed.testingAliases[alias] = true
			}
		}
		for _, declaration := range file.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok {
				parsed.functions[function.Name.Name] = append(
					parsed.functions[function.Name.Name],
					function,
				)
			}
		}
	}
	return parsed, nil
}

func s7LocateObservedTargetBody(
	parsed s7ObservedPackageAST,
	target s7FixtureTarget,
) (*ast.File, *ast.BlockStmt, *ast.Ident, error) {
	candidates := parsed.functions[target.test]
	if len(candidates) != 1 {
		return nil, nil, nil, fmt.Errorf(
			"%s resolves to %d declarations", target.test, len(candidates),
		)
	}
	function := candidates[0]
	receiver, err := s7TestingReceiver(function, parsed.testingAliases)
	if err != nil {
		return nil, nil, nil, err
	}
	body := function.Body
	bodyReceiver := receiver
	if target.subtest != "" {
		for _, segment := range strings.Split(target.subtest, "/") {
			bodies := s7SelectSubtestBodies(
				body,
				bodyReceiver,
				parsed.functions,
				parsed.testingAliases,
				segment,
			)
			if len(bodies) == 0 {
				bodies = s7SelectObservedBodiesDeep(
					body,
					bodyReceiver,
					parsed.functions,
					parsed.testingAliases,
					segment,
				)
			}
			if len(bodies) != 1 {
				return nil, nil, nil, fmt.Errorf(
					"%s/%s resolves %q to %d bodies",
					target.test, target.subtest, segment, len(bodies),
				)
			}
			body = bodies[0].body
			bodyReceiver = bodies[0].receiver
		}
	}
	if err := s7AssertRunnableBody(body, bodyReceiver); err != nil {
		return nil, nil, nil, err
	}
	for _, file := range parsed.files {
		if file.Pos() <= body.Pos() && body.End() <= file.End() {
			return file, body, bodyReceiver, nil
		}
	}
	return nil, nil, nil, fmt.Errorf("%s body has no source file", target.test)
}

func s7SelectObservedBodiesDeep(
	body *ast.BlockStmt,
	receiver *ast.Ident,
	functions map[string][]*ast.FuncDecl,
	testingAliases map[string]bool,
	subtest string,
) []s7SelectedBody {
	var bodies []s7SelectedBody
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !s7RunCallOnReceiver(call, receiver) {
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err == nil && value == subtest {
			bodies = append(
				bodies,
				s7CallbackBodies(call.Args[1], functions, testingAliases)...,
			)
		}
		return true
	})
	return bodies
}

func s7ObservedMarkerStatement(
	receiver string,
	expectedTest string,
	markerPath string,
	correlationToken string,
) ast.Stmt {
	return &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X: &ast.CallExpr{Fun: &ast.SelectorExpr{
				X: ast.NewIdent(receiver), Sel: ast.NewIdent("Name"),
			}},
			Op: token.EQL,
			Y:  &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(expectedTest)},
		},
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{
			X: &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: ast.NewIdent("s7leafmarker"), Sel: ast.NewIdent("Emit"),
				},
				Args: []ast.Expr{
					&ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(markerPath)},
					&ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(correlationToken)},
				},
			},
		}}},
	}
}

func s7AssertObservedLiteralMarker(
	body *ast.BlockStmt,
	receiver string,
	expectedTest string,
	markerPath string,
	correlationToken string,
) error {
	count := 0
	s7InspectRegistrationSyntax(body, func(node ast.Node) bool {
		statement, ok := node.(*ast.IfStmt)
		if !ok || len(statement.Body.List) != 1 {
			return true
		}
		condition, ok := statement.Cond.(*ast.BinaryExpr)
		if !ok || condition.Op != token.EQL {
			return true
		}
		nameCall, ok := condition.X.(*ast.CallExpr)
		if !ok {
			return true
		}
		nameSelector, ok := nameCall.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		nameBase, baseOK := nameSelector.X.(*ast.Ident)
		expected, expectedOK := s7ObservedStringLiteral(condition.Y)
		expression, ok := statement.Body.List[0].(*ast.ExprStmt)
		if !baseOK || nameBase.Name != receiver ||
			nameSelector.Sel.Name != "Name" ||
			!expectedOK || expected != expectedTest || !ok {
			return true
		}
		call, ok := expression.X.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		base, baseOK := selector.X.(*ast.Ident)
		if !baseOK || base.Name != "s7leafmarker" ||
			selector.Sel.Name != "Emit" {
			return true
		}
		pathValue, pathOK := s7ObservedStringLiteral(call.Args[0])
		tokenValue, tokenOK := s7ObservedStringLiteral(call.Args[1])
		if pathOK && tokenOK &&
			pathValue == markerPath && tokenValue == correlationToken {
			count++
		}
		return true
	})
	if count != 1 {
		return fmt.Errorf(
			"selected leaf has %d exact literal marker calls, want 1", count,
		)
	}
	return nil
}

func s7ObservedStringLiteral(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}

func s7AddObservedMarkerImport(file *ast.File) error {
	const markerImport = "github.com/tesseracode/tesserapatch/internal/s7marker"
	for _, imported := range file.Imports {
		pathValue, err := strconv.Unquote(imported.Path.Value)
		if err == nil && pathValue == markerImport {
			return fmt.Errorf("source already imports S7 marker helper")
		}
	}
	specification := &ast.ImportSpec{
		Name: ast.NewIdent("s7leafmarker"),
		Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(markerImport)},
	}
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.IMPORT || !general.Lparen.IsValid() {
			continue
		}
		general.Specs = append(general.Specs, specification)
		file.Imports = append(file.Imports, specification)
		return nil
	}
	file.Decls = append([]ast.Decl{&ast.GenDecl{
		Tok: token.IMPORT, Specs: []ast.Spec{specification},
	}}, file.Decls...)
	file.Imports = append(file.Imports, specification)
	return nil
}

func s7ObservedInnerTimeout(ctx context.Context, limit time.Duration) time.Duration {
	inner := limit
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if candidate := remaining / 3; candidate < inner {
			inner = candidate
		}
	}
	if inner < 500*time.Millisecond {
		return 500 * time.Millisecond
	}
	return inner.Round(time.Millisecond)
}

func s7ObservedInnerTimeoutWithCleanup(
	ctx context.Context,
	limit time.Duration,
	cleanupMargin time.Duration,
) time.Duration {
	inner := limit
	if deadline, ok := ctx.Deadline(); ok {
		candidate := time.Until(deadline) - cleanupMargin
		if candidate < inner {
			inner = candidate
		}
	}
	if inner < 500*time.Millisecond {
		return 500 * time.Millisecond
	}
	return inner.Round(time.Millisecond)
}

func s7CreateObservedWorkspace(repoRoot string) (string, error) {
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve observed-workspace parent: %w", err)
	}
	if err := os.MkdirAll(cacheRoot, 0o700); err != nil {
		return "", fmt.Errorf("create observed-workspace parent: %w", err)
	}
	workspace, err := os.MkdirTemp(cacheRoot, ".")
	if err != nil {
		return "", fmt.Errorf("create observed workspace: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(workspace)
		}
	}()
	if err := os.Chmod(workspace, 0o700); err != nil {
		return "", fmt.Errorf("mode observed workspace: %w", err)
	}
	info, err := os.Stat(workspace)
	if err != nil {
		return "", fmt.Errorf("stat observed workspace: %w", err)
	}
	if info.Mode().Perm() != 0o700 {
		return "", fmt.Errorf(
			"observed workspace mode = %04o, want 0700",
			info.Mode().Perm(),
		)
	}
	outside, err := s7ObservedPathOutside(repoRoot, workspace)
	if err != nil {
		return "", err
	}
	if !outside {
		return "", fmt.Errorf("observed workspace is inside repository")
	}
	base := strings.ToLower(filepath.Base(workspace))
	if strings.Contains(base, "s7") ||
		strings.Contains(base, "marker") ||
		strings.Contains(base, "overlay") {
		return "", fmt.Errorf("observed workspace name is not neutral")
	}
	cleanup = false
	return workspace, nil
}

func s7ObservedPathOutside(repoRoot, candidate string) (bool, error) {
	root, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return false, fmt.Errorf("resolve repository path: %w", err)
	}
	pathValue, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return false, fmt.Errorf("resolve observed workspace path: %w", err)
	}
	relative, err := filepath.Rel(root, pathValue)
	if err != nil {
		return false, fmt.Errorf("compare observed workspace path: %w", err)
	}
	return relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)), nil
}

func s7ObservedRandomHex(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate observed correlation token: %w", err)
	}
	return hex.EncodeToString(data), nil
}

func s7ValidateObservedMarkers(
	directory string,
	expectations map[string]s7ObservedMarkerExpectation,
) []string {
	var failures []string
	expectedFiles := map[string]bool{}
	for key, expectation := range expectations {
		name := filepath.Base(expectation.path)
		expectedFiles[name] = true
		if filepath.Dir(expectation.path) != directory {
			failures = append(failures, key+" marker escaped correlation directory")
			continue
		}
		data, err := os.ReadFile(expectation.path)
		if err != nil || string(data) != expectation.token {
			failures = append(failures, fmt.Sprintf("%s marker=%q err=%v", key, data, err))
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		failures = append(failures, err.Error())
		return failures
	}
	for _, entry := range entries {
		if entry.IsDir() || !expectedFiles[entry.Name()] {
			failures = append(failures, "unexpected marker "+entry.Name())
		}
	}
	sort.Strings(failures)
	return failures
}

func s7AssertObservedProcessReaped(workspace string) error {
	data, err := os.ReadFile(filepath.Join(workspace, "pid"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return err
	}
	deadline := time.Now().Add(time.Second)
	for s7ObservedProcessAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if s7ObservedProcessAlive(pid) {
		s7KillObservedPID(pid)
		return fmt.Errorf("observed registration descendant %d is still alive", pid)
	}
	return nil
}

func s7ObservedTopLevelPattern(testNames []string) string {
	escaped := make([]string, len(testNames))
	for index, name := range testNames {
		escaped[index] = regexp.QuoteMeta(name)
	}
	sort.Strings(escaped)
	return "^(?:" + strings.Join(escaped, "|") + ")$"
}

func s7ObservedTargetKey(target s7FixtureTarget) string {
	return s7ObservedPackagePath(target.dir) + "/" + s7ObservedFullTestName(target)
}

func s7ObservedFullTestName(target s7FixtureTarget) string {
	if target.subtest == "" {
		return target.test
	}
	return target.test + "/" + target.subtest
}

func s7ObservedPackagePath(directory string) string {
	return "github.com/tesseracode/tesserapatch/" + filepath.ToSlash(directory)
}

func s7SortedObservedKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
