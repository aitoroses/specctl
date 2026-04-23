package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type DeltaStatus string

type DeltaIntent string

const (
	DeltaStatusOpen       DeltaStatus = "open"
	DeltaStatusInProgress DeltaStatus = "in-progress"
	DeltaStatusClosed     DeltaStatus = "closed"
	DeltaStatusDeferred   DeltaStatus = "deferred"
	DeltaStatusWithdrawn  DeltaStatus = "withdrawn"

	DeltaIntentAdd    DeltaIntent = "add"
	DeltaIntentChange DeltaIntent = "change"
	DeltaIntentRemove DeltaIntent = "remove"
	DeltaIntentRepair DeltaIntent = "repair"
)

var deltaIDPattern = regexp.MustCompile(`^D-(\d{3})$`)

type Delta struct {
	ID                  string      `yaml:"id" json:"id"`
	Area                string      `yaml:"area" json:"area"`
	Intent              DeltaIntent `yaml:"intent,omitempty" json:"intent,omitempty"`
	Status              DeltaStatus `yaml:"status" json:"status"`
	OriginCheckpoint    string      `yaml:"origin_checkpoint" json:"origin_checkpoint"`
	Current             string      `yaml:"current" json:"current"`
	Target              string      `yaml:"target" json:"target"`
	Notes               string      `yaml:"notes" json:"notes"`
	AffectsRequirements []string    `yaml:"affects_requirements,omitempty" json:"affects_requirements,omitempty"`
	Updates             []string    `yaml:"updates,omitempty" json:"updates,omitempty"`
	WithdrawnReason     string      `yaml:"withdrawn_reason,omitempty" json:"withdrawn_reason,omitempty"`
}

func NewDelta(id string, intent DeltaIntent, area, originCheckpoint, current, target, notes string, affectsRequirements []string) (Delta, error) {
	delta := Delta{
		ID:                  id,
		Area:                area,
		Intent:              intent,
		Status:              DeltaStatusOpen,
		OriginCheckpoint:    originCheckpoint,
		Current:             current,
		Target:              target,
		Notes:               notes,
		AffectsRequirements: append([]string{}, affectsRequirements...),
		Updates:             deltaUpdatesForIntent(intent),
	}
	if err := validateDelta(delta); err != nil {
		return Delta{}, err
	}
	return delta, nil
}

func IsValidDeltaStatus(s string) bool {
	switch DeltaStatus(s) {
	case DeltaStatusOpen, DeltaStatusInProgress, DeltaStatusClosed, DeltaStatusDeferred, DeltaStatusWithdrawn:
		return true
	default:
		return false
	}
}

func IsValidDeltaID(id string) bool {
	return deltaIDPattern.MatchString(id)
}

func IsValidDeltaIntent(intent string) bool {
	switch DeltaIntent(intent) {
	case DeltaIntentAdd, DeltaIntentChange, DeltaIntentRemove, DeltaIntentRepair:
		return true
	default:
		return false
	}
}

func deltaUpdatesForIntent(intent DeltaIntent) []string {
	switch intent {
	case DeltaIntentChange:
		return []string{"replace_requirement"}
	case DeltaIntentRemove:
		return []string{"withdraw_requirement"}
	case DeltaIntentRepair:
		return []string{"stale_requirement"}
	default:
		return []string{"add_requirement"}
	}
}

func ParseDeltaIDNumber(id string) (int, error) {
	matches := deltaIDPattern.FindStringSubmatch(id)
	if matches == nil {
		return 0, fmt.Errorf("invalid delta ID format: %s (expected D-NNN)", id)
	}

	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid delta ID number: %s", id)
	}
	return n, nil
}

func ExpectedDeltaID(n int) string {
	return fmt.Sprintf("D-%03d", n)
}

func ValidateDeltaSequence(deltas []Delta) error {
	for i, delta := range deltas {
		expected := ExpectedDeltaID(i + 1)
		if delta.ID != expected {
			return fmt.Errorf("delta IDs must be sequential without gaps; expected %s, found %s", expected, delta.ID)
		}
		if err := validateDelta(delta); err != nil {
			return err
		}
	}
	return nil
}

func NextDeltaID(deltas []Delta) (string, error) {
	if err := ValidateDeltaSequence(deltas); err != nil {
		return "", err
	}
	return ExpectedDeltaID(len(deltas) + 1), nil
}

func validateDelta(delta Delta) error {
	if !IsValidDeltaID(delta.ID) {
		return fmt.Errorf("delta ID %q must match D-NNN", delta.ID)
	}
	if strings.TrimSpace(delta.Area) == "" {
		return fmt.Errorf("delta %s area is required", delta.ID)
	}
	if !shaPattern.MatchString(delta.OriginCheckpoint) {
		return fmt.Errorf("delta %s origin_checkpoint must be a git SHA", delta.ID)
	}
	if strings.TrimSpace(delta.Current) == "" {
		return fmt.Errorf("delta %s current is required", delta.ID)
	}
	if strings.TrimSpace(delta.Target) == "" {
		return fmt.Errorf("delta %s target is required", delta.ID)
	}
	if !IsValidDeltaStatus(string(delta.Status)) {
		return fmt.Errorf("delta %s has invalid status %q", delta.ID, delta.Status)
	}
	if delta.Intent != "" && !IsValidDeltaIntent(string(delta.Intent)) {
		return fmt.Errorf("delta %s has invalid intent %q", delta.ID, delta.Intent)
	}
	if delta.Status == DeltaStatusWithdrawn {
		if strings.TrimSpace(delta.WithdrawnReason) == "" {
			return fmt.Errorf("delta %s withdrawn_reason is required when status is withdrawn", delta.ID)
		}
	} else if strings.TrimSpace(delta.WithdrawnReason) != "" {
		return fmt.Errorf("delta %s withdrawn_reason is only valid when status is withdrawn", delta.ID)
	}
	return nil
}
