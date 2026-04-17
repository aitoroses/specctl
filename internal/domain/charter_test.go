package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestCharterValidateAndOrder(t *testing.T) {
	charter := &Charter{
		Name:        "runtime",
		Title:       "Runtime System",
		Description: "Specs for runtime control-plane and data-plane behavior",
		Groups: []CharterGroup{
			{Key: "execution", Title: "Execution Engine", Order: 10},
			{Key: "recovery", Title: "Recovery and Cleanup", Order: 20},
		},
		Specs: []CharterSpecEntry{
			{Slug: "session-lifecycle", Group: "recovery", Order: 20, DependsOn: []string{"redis-state"}, Notes: "Session FSM and cleanup behavior"},
			{Slug: "redis-state", Group: "execution", Order: 10, DependsOn: []string{}, Notes: "Storage and CAS guarantees"},
			{Slug: "recovery-projection", Group: "recovery", Order: 10, DependsOn: []string{"redis-state"}, Notes: "Recovery planning order"},
		},
	}

	if err := charter.Validate(); err != nil {
		t.Fatalf("expected valid charter, got %v", err)
	}

	ordered, err := charter.OrderedSpecs()
	if err != nil {
		t.Fatalf("ordered specs: %v", err)
	}
	got := []string{ordered[0].Slug, ordered[1].Slug, ordered[2].Slug}
	want := []string{"redis-state", "recovery-projection", "session-lifecycle"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ordered specs = %v, want %v", got, want)
	}
}

func TestCharterValidateRejectsV2SchemaViolations(t *testing.T) {
	tests := []struct {
		name    string
		charter *Charter
		want    string
	}{
		{
			name: "description required",
			charter: &Charter{
				Name:   "runtime",
				Title:  "Runtime System",
				Groups: []CharterGroup{{Key: "execution", Title: "Execution Engine", Order: 10}},
				Specs:  []CharterSpecEntry{{Slug: "redis-state", Group: "execution", Order: 10, DependsOn: []string{}, Notes: "Storage and CAS guarantees"}},
			},
			want: "description is required",
		},
		{
			name: "notes required",
			charter: &Charter{
				Name:        "runtime",
				Title:       "Runtime System",
				Description: "Specs",
				Groups:      []CharterGroup{{Key: "execution", Title: "Execution Engine", Order: 10}},
				Specs:       []CharterSpecEntry{{Slug: "redis-state", Group: "execution", Order: 10, DependsOn: []string{}, Notes: ""}},
			},
			want: "notes are required",
		},
		{
			name: "self dependency rejected",
			charter: &Charter{
				Name:        "runtime",
				Title:       "Runtime System",
				Description: "Specs",
				Groups:      []CharterGroup{{Key: "execution", Title: "Execution Engine", Order: 10}},
				Specs:       []CharterSpecEntry{{Slug: "redis-state", Group: "execution", Order: 10, DependsOn: []string{"redis-state"}, Notes: "Storage and CAS guarantees"}},
			},
			want: "cannot depend on itself",
		},
		{
			name: "dependency must stay within charter slug namespace",
			charter: &Charter{
				Name:        "runtime",
				Title:       "Runtime System",
				Description: "Specs",
				Groups:      []CharterGroup{{Key: "execution", Title: "Execution Engine", Order: 10}},
				Specs: []CharterSpecEntry{
					{Slug: "redis-state", Group: "execution", Order: 10, DependsOn: []string{}, Notes: "Storage and CAS guarantees"},
					{Slug: "session-lifecycle", Group: "execution", Order: 20, DependsOn: []string{"runtime:redis-state"}, Notes: "Session FSM and cleanup behavior"},
				},
			},
			want: `depends on unknown spec "runtime:redis-state"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.charter.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
		})
	}
}

func TestCharterValidateUnknownDependencyDoesNotReportCycle(t *testing.T) {
	charter := &Charter{
		Name:        "runtime",
		Title:       "Runtime System",
		Description: "Specs for runtime control-plane and data-plane behavior",
		Groups: []CharterGroup{
			{Key: "execution", Title: "Execution Engine", Order: 10},
		},
		Specs: []CharterSpecEntry{
			{Slug: "redis-state", Group: "execution", Order: 10, DependsOn: []string{"missing-spec"}, Notes: "Storage and CAS guarantees"},
		},
	}

	err := charter.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want dependency validation error")
	}

	var validationErr *CharterValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Validate() error type = %T, want *CharterValidationError", err)
	}
	if len(validationErr.Messages) != 1 || !strings.Contains(validationErr.Messages[0], `depends on unknown spec "missing-spec"`) {
		t.Fatalf("validationErr.Messages = %#v", validationErr.Messages)
	}
	if strings.Contains(err.Error(), "dependency cycle detected") {
		t.Fatalf("Validate() error = %v, want no cycle classification", err)
	}

	ordered, orderErr := charter.OrderedSpecs()
	if orderErr != nil {
		t.Fatalf("OrderedSpecs() error = %v, want nil", orderErr)
	}
	if len(ordered) != 1 || ordered[0].Slug != "redis-state" {
		t.Fatalf("ordered = %#v", ordered)
	}
}

func TestCharterOrderedSpecsUsesStableTieBreaks(t *testing.T) {
	charter := &Charter{
		Name:        "runtime",
		Title:       "Runtime System",
		Description: "Specs for runtime control-plane and data-plane behavior",
		Groups: []CharterGroup{
			{Key: "execution", Title: "Execution Engine", Order: 10},
			{Key: "recovery", Title: "Recovery and Cleanup", Order: 20},
		},
		Specs: []CharterSpecEntry{
			{Slug: "beta", Group: "execution", Order: 10, DependsOn: []string{}, Notes: "Beta"},
			{Slug: "alpha", Group: "execution", Order: 10, DependsOn: []string{}, Notes: "Alpha"},
			{Slug: "omega", Group: "recovery", Order: 10, DependsOn: []string{}, Notes: "Omega"},
		},
	}

	ordered, err := charter.OrderedSpecs()
	if err != nil {
		t.Fatalf("OrderedSpecs: %v", err)
	}

	got := []string{ordered[0].Slug, ordered[1].Slug, ordered[2].Slug}
	want := []string{"alpha", "beta", "omega"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ordered specs = %v, want %v", got, want)
	}
}

func TestCharterOrderedSpecsCycleReturnsTypedErrorAndLenientFallback(t *testing.T) {
	charter := &Charter{
		Name:        "runtime",
		Title:       "Runtime System",
		Description: "Specs for runtime control-plane and data-plane behavior",
		Groups: []CharterGroup{
			{Key: "execution", Title: "Execution Engine", Order: 10},
			{Key: "recovery", Title: "Recovery and Cleanup", Order: 20},
		},
		Specs: []CharterSpecEntry{
			{Slug: "redis-state", Group: "execution", Order: 10, DependsOn: []string{"session-lifecycle"}, Notes: "Storage and CAS guarantees"},
			{Slug: "session-lifecycle", Group: "recovery", Order: 20, DependsOn: []string{"redis-state"}, Notes: "Session FSM and cleanup behavior"},
			{Slug: "recovery-projection", Group: "recovery", Order: 10, DependsOn: []string{"redis-state"}, Notes: "Recovery planning order"},
		},
	}

	_, err := charter.OrderedSpecs()
	if err == nil {
		t.Fatal("OrderedSpecs() error = nil, want cycle error")
	}

	var cycleErr *CharterCycleError
	if !errors.As(err, &cycleErr) {
		t.Fatalf("OrderedSpecs() error type = %T, want *CharterCycleError", err)
	}
	if !reflect.DeepEqual(cycleErr.Slugs, []string{"redis-state", "session-lifecycle"}) {
		t.Fatalf("cycleErr.Slugs = %#v", cycleErr.Slugs)
	}

	ordered := OrderedCharterSpecsLenient(charter)
	got := []string{ordered[0].Slug, ordered[1].Slug, ordered[2].Slug}
	want := []string{"redis-state", "recovery-projection", "session-lifecycle"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lenient ordered specs = %v, want %v", got, want)
	}
}

func TestBuildLenientCharterOrderingSharesSpecsAndIndex(t *testing.T) {
	charter := &Charter{
		Name:        "runtime",
		Title:       "Runtime System",
		Description: "Specs for runtime control-plane and data-plane behavior",
		Groups: []CharterGroup{
			{Key: "execution", Title: "Execution Engine", Order: 10},
			{Key: "recovery", Title: "Recovery and Cleanup", Order: 20},
		},
		Specs: []CharterSpecEntry{
			{Slug: "redis-state", Group: "execution", Order: 10, DependsOn: []string{"session-lifecycle"}, Notes: "Storage and CAS guarantees"},
			{Slug: "session-lifecycle", Group: "recovery", Order: 20, DependsOn: []string{"redis-state"}, Notes: "Session FSM and cleanup behavior"},
			{Slug: "recovery-projection", Group: "recovery", Order: 10, DependsOn: []string{"redis-state"}, Notes: "Recovery planning order"},
		},
	}

	ordering := BuildLenientCharterOrdering(charter)
	gotSpecs := []string{ordering.Specs[0].Slug, ordering.Specs[1].Slug, ordering.Specs[2].Slug}
	wantSpecs := []string{"redis-state", "recovery-projection", "session-lifecycle"}
	if !reflect.DeepEqual(gotSpecs, wantSpecs) {
		t.Fatalf("ordering.Specs = %v, want %v", gotSpecs, wantSpecs)
	}

	wantIndex := map[string]int{
		"redis-state":         0,
		"recovery-projection": 1,
		"session-lifecycle":   2,
	}
	if !reflect.DeepEqual(ordering.Index, wantIndex) {
		t.Fatalf("ordering.Index = %#v, want %#v", ordering.Index, wantIndex)
	}
}

func TestCharterValidationReturnsTypedErrors(t *testing.T) {
	_, err := NewCharterSpecEntry("session-lifecycle", "recovery", -1, []string{}, "Session FSM and cleanup behavior")
	if err == nil {
		t.Fatal("NewCharterSpecEntry() error = nil, want typed validation error")
	}

	var validationErr *CharterValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("NewCharterSpecEntry() error type = %T, want *CharterValidationError", err)
	}
	if !reflect.DeepEqual(validationErr.Messages, []string{`spec "session-lifecycle" order must be >= 0`}) {
		t.Fatalf("validationErr.Messages = %#v", validationErr.Messages)
	}
}

func TestCharterReplaceSpecEntryUsesWholeEntryReplacement(t *testing.T) {
	charter := &Charter{
		Name:        "runtime",
		Title:       "Runtime System",
		Description: "Specs for runtime control-plane and data-plane behavior",
		Groups: []CharterGroup{
			{Key: "execution", Title: "Execution Engine", Order: 10},
			{Key: "recovery", Title: "Recovery and Cleanup", Order: 20},
		},
		Specs: []CharterSpecEntry{
			{Slug: "session-lifecycle", Group: "recovery", Order: 20, DependsOn: []string{"redis-state"}, Notes: "Session FSM and cleanup behavior"},
			{Slug: "redis-state", Group: "execution", Order: 10, DependsOn: []string{}, Notes: "Storage and CAS guarantees"},
		},
	}

	entry, err := NewCharterSpecEntry("session-lifecycle", "recovery", 30, []string{}, "Replacement notes")
	if err != nil {
		t.Fatalf("NewCharterSpecEntry: %v", err)
	}
	if err := charter.ReplaceSpecEntry(entry); err != nil {
		t.Fatalf("ReplaceSpecEntry: %v", err)
	}

	replaced := charter.SpecBySlug("session-lifecycle")
	if replaced == nil {
		t.Fatal("expected replaced entry to exist")
	}
	if replaced.Order != 30 || !reflect.DeepEqual(replaced.DependsOn, []string{}) || replaced.Notes != "Replacement notes" {
		t.Fatalf("replacement did not overwrite the full entry: %+v", replaced)
	}
}

func TestCharterEnsureGroupAndTrackingMembershipHelpers(t *testing.T) {
	charter := &Charter{
		Name:        "runtime",
		Title:       "Runtime System",
		Description: "Specs for runtime control-plane and data-plane behavior",
		Groups:      []CharterGroup{{Key: "execution", Title: "Execution Engine", Order: 10}},
		Specs:       []CharterSpecEntry{{Slug: "redis-state", Group: "execution", Order: 10, DependsOn: []string{}, Notes: "Storage and CAS guarantees"}},
	}

	if err := charter.EnsureGroup(CharterGroup{Key: "recovery", Title: "Recovery and Cleanup", Order: 20}); err != nil {
		t.Fatalf("EnsureGroup: %v", err)
	}
	if err := charter.EnsureGroup(CharterGroup{Key: "execution", Title: "Ignored duplicate", Order: 99}); err != nil {
		t.Fatalf("EnsureGroup existing: %v", err)
	}

	missing := charter.MissingTrackingSpecs([]string{"redis-state"})
	if !reflect.DeepEqual(missing, []string{}) {
		t.Fatalf("missing = %v, want none", missing)
	}

	entry, err := NewCharterSpecEntry("session-lifecycle", "recovery", 20, []string{"redis-state"}, "Session FSM and cleanup behavior")
	if err != nil {
		t.Fatalf("NewCharterSpecEntry: %v", err)
	}
	if err := charter.ReplaceSpecEntry(entry); err != nil {
		t.Fatalf("ReplaceSpecEntry add: %v", err)
	}

	missing = charter.MissingTrackingSpecs([]string{"redis-state"})
	if !reflect.DeepEqual(missing, []string{"session-lifecycle"}) {
		t.Fatalf("missing = %v, want %v", missing, []string{"session-lifecycle"})
	}

	extra := charter.ExtraTrackingSpecs([]string{"redis-state", "rogue-spec"})
	if !reflect.DeepEqual(extra, []string{"rogue-spec"}) {
		t.Fatalf("extra = %v, want %v", extra, []string{"rogue-spec"})
	}
}
