package domain

import "testing"

func TestIsValidDeltaID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"D-001", true},
		{"D-010", true},
		{"D-999", true},
		{"D-01", false},
		{"D-0001", false},
		{"D001", false},
		{"d-001", false},
		{"X-001", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsValidDeltaID(tt.input)
			if got != tt.want {
				t.Errorf("IsValidDeltaID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDeltaIDNumber(t *testing.T) {
	n, err := ParseDeltaIDNumber("D-042")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 42 {
		t.Errorf("expected 42, got %d", n)
	}

	_, err = ParseDeltaIDNumber("invalid")
	if err == nil {
		t.Error("expected error for invalid ID")
	}
}

func TestExpectedDeltaID(t *testing.T) {
	if got := ExpectedDeltaID(1); got != "D-001" {
		t.Errorf("expected D-001, got %s", got)
	}
	if got := ExpectedDeltaID(42); got != "D-042" {
		t.Errorf("expected D-042, got %s", got)
	}
}

func TestIsValidDeltaStatus(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"open", true},
		{"in-progress", true},
		{"closed", true},
		{"deferred", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsValidDeltaStatus(tt.input)
			if got != tt.want {
				t.Errorf("IsValidDeltaStatus(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewDeltaStartsOpen(t *testing.T) {
	delta, err := NewDelta("D-009", DeltaIntentAdd, "Recovery handshake", "a1b2c3f", "Current gap", "Target state", "", nil)
	if err != nil {
		t.Fatalf("NewDelta: %v", err)
	}
	if delta.Status != DeltaStatusOpen {
		t.Fatalf("status = %s, want %s", delta.Status, DeltaStatusOpen)
	}
	if delta.OriginCheckpoint != "a1b2c3f" {
		t.Fatalf("origin_checkpoint = %q", delta.OriginCheckpoint)
	}
}
