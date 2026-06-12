package lmu

import "testing"

func TestEventAtOKReturnsLastEventAtOrBeforeSample(t *testing.T) {
	data := newImportedTelemetry()
	data.events["Gear"] = []EventRow{
		{Ts: 0.25, FloatValue: 2},
		{Ts: 0.50, FloatValue: 3},
		{Ts: 1.00, FloatValue: 4},
	}

	tests := []struct {
		name       string
		sampleTime float64
		want       float64
		wantOK     bool
	}{
		{"before first event", 0.10, 0, false},
		{"exact first event", 0.25, 2, true},
		{"between events", 0.75, 3, true},
		{"after last event", 1.25, 4, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := data.eventAtOK("Gear", tt.sampleTime)
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("eventAtOK() = (%v, %v), want (%v, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestIntEventWheelAtReturnsLastRoundedEventWheelAtOrBeforeSample(t *testing.T) {
	data := newImportedTelemetry()
	data.eventWheels["WheelsDetached"] = []EventWheelRow{
		{Ts: 0.25, V1: 0, V2: 0, V3: 0, V4: 0},
		{Ts: 0.50, V1: 1.2, V2: 1.6, V3: 2.4, V4: 2.6},
	}

	if got := data.intEventWheelAt("WheelsDetached", 0.10); got != [4]int64{} {
		t.Fatalf("before first event = %v, want zeros", got)
	}

	got := data.intEventWheelAt("WheelsDetached", 0.75)
	want := [4]int64{1, 2, 2, 3}
	if got != want {
		t.Fatalf("after second event = %v, want %v", got, want)
	}
}
