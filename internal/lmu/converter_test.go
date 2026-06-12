package lmu

import (
	"testing"

	"github.com/La-Pace/lapace-core/contract/session"
	"github.com/stretchr/testify/assert"
)

func TestDerivePhase(t *testing.T) {
	tests := []struct {
		sessionType string
		phase       session.Phase
		exact       bool
	}{
		{"Practice", session.PhasePractice, true},
		{"Qualify", session.PhaseQualifying, true},
		{"Race", session.PhaseRace, true},
		{"Unknown", session.PhaseTesting, false},
	}

	for _, tt := range tests {
		t.Run(tt.sessionType, func(t *testing.T) {
			phase, exact := DerivePhase(tt.sessionType)
			assert.Equal(t, tt.phase, phase)
			assert.Equal(t, tt.exact, exact)
		})
	}
}

func TestParseRecordingTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{
			name:  "ISO format with underscores",
			input: "2026-05-03T21_23_18Z",
			want:  1777843398.0,
		},
		{
			name:  "standard ISO format",
			input: "2026-03-24T02:10:46Z",
			want:  1774318246.0,
		},
		{
			name:    "invalid format",
			input:   "not-a-date",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRecordingTime(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseRecordingTime(%q) expected error, got %f", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRecordingTime(%q) unexpected error: %v", tt.input, err)
			}
			// Allow 1-second tolerance for floating point
			if diff := got - tt.want; diff < -1 || diff > 1 {
				t.Errorf("ParseRecordingTime(%q) = %f, want ~%f (diff=%f)", tt.input, got, tt.want, diff)
			}
		})
	}
}
