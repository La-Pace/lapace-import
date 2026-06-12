package lmu

import (
	"fmt"
	"strings"
	"time"

	"github.com/La-Pace/lapace-core/contract/session"
)

// DerivePhase converts an LMU SessionType metadata value to a Lapace phase name.
func DerivePhase(sessionType string) (session.Phase, bool) {
	switch sessionType {
	case "Practice":
		return session.PhasePractice, true
	case "Qualify":
		return session.PhaseQualifying, true
	case "Race":
		return session.PhaseRace, true
	default:
		return session.PhaseTesting, false
	}
}

// ParseRecordingTime converts an LMU RecordingTime string to epoch seconds.
// LMU uses underscores in timestamps (e.g., "2026-05-03T21_23_18Z")
// which are not valid ISO 8601. This function normalizes them before parsing.
func ParseRecordingTime(recordingTime string) (float64, error) {
	t, err := ParseRecordingTimeAsTime(recordingTime)
	if err != nil {
		return 0, err
	}
	return float64(t.Unix()), nil
}

// ParseRecordingTimeAsTime converts an LMU RecordingTime string to a time.Time.
func ParseRecordingTimeAsTime(recordingTime string) (time.Time, error) {
	normalized := strings.ReplaceAll(recordingTime, "_", ":")
	normalized = strings.Replace(normalized, "T:", "T", 1)

	t, err := time.Parse(time.RFC3339, normalized)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse RecordingTime %q: %w", recordingTime, err)
	}
	return t, nil
}

// ConvertWearFraction converts raw LMU tyre wear (0-100 remaining life %) to
// contract wear fraction (0.0 = new, 1.0 = completely worn).
func ConvertWearFraction(r WheelRow) WheelRow {
	return WheelRow{
		V1: (100.0 - r.V1) / 100.0,
		V2: (100.0 - r.V2) / 100.0,
		V3: (100.0 - r.V3) / 100.0,
		V4: (100.0 - r.V4) / 100.0,
	}
}
