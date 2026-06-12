package lmu

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/La-Pace/lapace-core/contract/telemetry"
)

func TestParseCarSetup_Minimal(t *testing.T) {
	// Minimal valid CarSetup JSON — only the non-VM/WM keys.
	input := `{"symmetric": true}`

	got, err := ParseCarSetup(input)
	if err != nil {
		t.Fatalf("ParseCarSetup: %v", err)
	}

	if !got.Symmetric {
		t.Error("Symmetric = false, want true")
	}

	// All value fields should be zero
	if got.Aero.FrontWing != 0 {
		t.Errorf("Aero.FrontWing = %v, want 0", got.Aero.FrontWing)
	}
}

func TestParseCarSetup_AeroAndBrakes(t *testing.T) {
	input := `{
		"VM_FRONT_WING": {"value": 12, "available": true, "isFreeSetting": true, "minValue": 0, "maxValue": 30},
		"VM_REAR_WING":  {"value": 8,  "available": true, "isFreeSetting": true, "minValue": 0, "maxValue": 30},
		"VM_BRAKE_BALANCE": {"value": 36, "available": true, "isFreeSetting": true, "minValue": 0, "maxValue": 57},
		"VM_BRAKE_DUCTS":   {"value": 4,  "available": true, "isFreeSetting": true, "minValue": 0, "maxValue": 10},
		"VM_BRAKE_DUCTS_REAR": {"value": 5, "available": true, "isFreeSetting": true, "minValue": 0, "maxValue": 10}
	}`

	got, err := ParseCarSetup(input)
	if err != nil {
		t.Fatalf("ParseCarSetup: %v", err)
	}

	// Aero
	if got.Aero.FrontWing != 12 {
		t.Errorf("Aero.FrontWing = %v, want 12", got.Aero.FrontWing)
	}
	if got.Aero.RearWing != 8 {
		t.Errorf("Aero.RearWing = %v, want 8", got.Aero.RearWing)
	}

	// Brakes
	if got.Brakes.Balance != 36 {
		t.Errorf("Brakes.Balance = %v, want 36", got.Brakes.Balance)
	}
	if got.Brakes.FrontDucts != 4 {
		t.Errorf("Brakes.FrontDucts = %v, want 4", got.Brakes.FrontDucts)
	}
	if got.Brakes.RearDucts != 5 {
		t.Errorf("Brakes.RearDucts = %v, want 5", got.Brakes.RearDucts)
	}
}

func TestParseCarSetup_Wheels(t *testing.T) {
	input := `{
		"WM_BRAKEDISC-W_FL":  {"value": 1, "available": true, "isFreeSetting": false, "minValue": 0, "maxValue": 1},
		"WM_BRAKEDISC-W_FR":  {"value": 2, "available": true, "isFreeSetting": false, "minValue": 0, "maxValue": 1},
		"WM_BRAKEDISC-W_RL":  {"value": 3, "available": true, "isFreeSetting": false, "minValue": 0, "maxValue": 1},
		"WM_BRAKEDISC-W_RR":  {"value": 4, "available": true, "isFreeSetting": false, "minValue": 0, "maxValue": 1},
		"WM_CAMBER-W_FL":     {"value": -3.5, "available": true, "isFreeSetting": true, "minValue": -10, "maxValue": 0},
		"WM_CAMBER-W_FR":     {"value": -3.2, "available": true, "isFreeSetting": true, "minValue": -10, "maxValue": 0},
		"WM_PRESSURE-W_FL":   {"value": 28.5, "available": true, "isFreeSetting": true, "minValue": 20, "maxValue": 40},
		"WM_SRUBBER-W_RR":    {"value": 1.5, "available": true, "isFreeSetting": true, "minValue": 0, "maxValue": 5}
	}`

	got, err := ParseCarSetup(input)
	if err != nil {
		t.Fatalf("ParseCarSetup: %v", err)
	}

	// FL = index 0
	if got.Wheels[0].BrakeDisc != 1 {
		t.Errorf("Wheels[0].BrakeDisc = %v, want 1", got.Wheels[0].BrakeDisc)
	}
	if got.Wheels[0].Camber != -3.5 {
		t.Errorf("Wheels[0].Camber = %v, want -3.5", got.Wheels[0].Camber)
	}
	if got.Wheels[0].Pressure != 28.5 {
		t.Errorf("Wheels[0].Pressure = %v, want 28.5", got.Wheels[0].Pressure)
	}

	// FR = index 1
	if got.Wheels[1].BrakeDisc != 2 {
		t.Errorf("Wheels[1].BrakeDisc = %v, want 2", got.Wheels[1].BrakeDisc)
	}
	if got.Wheels[1].Camber != -3.2 {
		t.Errorf("Wheels[1].Camber = %v, want -3.2", got.Wheels[1].Camber)
	}

	// RL = index 2
	if got.Wheels[2].BrakeDisc != 3 {
		t.Errorf("Wheels[2].BrakeDisc = %v, want 3", got.Wheels[2].BrakeDisc)
	}

	// RR = index 3
	if got.Wheels[3].BrakeDisc != 4 {
		t.Errorf("Wheels[3].BrakeDisc = %v, want 4", got.Wheels[3].BrakeDisc)
	}

	// ScrubRubber (LMU typo "SRUBBER" → game-agnostic "ScrubRubber")
	if got.Wheels[3].ScrubRubber != 1.5 {
		t.Errorf("Wheels[3].ScrubRubber = %v, want 1.5", got.Wheels[3].ScrubRubber)
	}
}

func TestParseCarSetup_GearGraph(t *testing.T) {
	input := `{
		"gearGraph": {
			"kiloRPM": [0, 0, 8.0, 6.1, 8.0, 6.6, 8.0],
			"numForwardGears": 6,
			"topSpeed": [0, 117, 152, 186, 221, 253, 287],
			"unit": "kph"
		}
	}`

	got, err := ParseCarSetup(input)
	if err != nil {
		t.Fatalf("ParseCarSetup: %v", err)
	}

	if got.GearGraph.NumForwardGears != 6 {
		t.Errorf("GearGraph.NumForwardGears = %d, want 6", got.GearGraph.NumForwardGears)
	}
	if got.GearGraph.Unit != "kph" {
		t.Errorf("GearGraph.Unit = %q, want \"kph\"", got.GearGraph.Unit)
	}
	if len(got.GearGraph.KiloRPM) != 7 {
		t.Errorf("len(GearGraph.KiloRPM) = %d, want 7", len(got.GearGraph.KiloRPM))
	}
	if len(got.GearGraph.TopSpeed) != 7 {
		t.Errorf("len(GearGraph.TopSpeed) = %d, want 7", len(got.GearGraph.TopSpeed))
	}
}

func TestParseCarSetup_RealFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/carsetup.json")
	if err != nil {
		t.Skip("testdata/carsetup.json not found")
	}

	got, err := ParseCarSetup(string(data))
	if err != nil {
		t.Fatalf("ParseCarSetup: %v", err)
	}

	// Verify basic structure
	if !got.Symmetric {
		t.Error("Symmetric = false, want true (fixture is a symmetric car)")
	}
	if got.GearGraph.NumForwardGears != 6 {
		t.Errorf("GearGraph.NumForwardGears = %d, want 6", got.GearGraph.NumForwardGears)
	}

	// Verify representative fields from the fixture are populated.
	// These fields are known to have non-zero values in the Bahrain fixture.
	if got.Aero.RearWing != 19 {
		t.Errorf("Aero.RearWing = %v, want 19", got.Aero.RearWing)
	}
	if got.Brakes.Balance != 36 {
		t.Errorf("Brakes.Balance = %v, want 36", got.Brakes.Balance)
	}
	if got.Engine.Mixture != 1 {
		t.Errorf("Engine.Mixture = %v, want 1", got.Engine.Mixture)
	}
	if got.Differential.Preload != 3 {
		t.Errorf("Differential.Preload = %v, want 3", got.Differential.Preload)
	}
	if got.Suspension.Front.AntiSway != 5 {
		t.Errorf("Suspension.Front.AntiSway = %v, want 5", got.Suspension.Front.AntiSway)
	}
	if got.Chassis.SteerLock != 6 {
		t.Errorf("Chassis.SteerLock = %v, want 6", got.Chassis.SteerLock)
	}

	// Verify all 4 wheels have camber data
	for i, w := range got.Wheels {
		if w.Camber == 0 {
			t.Errorf("Wheels[%d].Camber = 0, expected non-zero from fixture", i)
		}
	}
}

func TestParseCarSetup_ParamMeta(t *testing.T) {
	input := `{
		"VM_FRONT_WING": {"value": 12, "available": true, "isFreeSetting": true, "minValue": 0, "maxValue": 30},
		"VM_REAR_WING":  {"value": 8,  "available": true, "isFreeSetting": false, "minValue": 0, "maxValue": 30},
		"WM_PRESSURE-W_FL": {"value": 28.5, "available": true, "isFreeSetting": true, "minValue": 20, "maxValue": 40},
		"WM_BRAKEDISC-W_RR": {"value": 3, "available": false, "isFreeSetting": false, "minValue": 0, "maxValue": 1}
	}`

	got, err := ParseCarSetup(input)
	if err != nil {
		t.Fatalf("ParseCarSetup: %v", err)
	}

	if len(got.ParamMeta) == 0 {
		t.Fatal("ParamMeta is empty, expected metadata entries")
	}

	// Verify aero.frontWing metadata
	meta, ok := got.ParamMeta["aero.frontWing"]
	if !ok {
		t.Fatal("ParamMeta missing key \"aero.frontWing\"")
	}
	if !meta.Available {
		t.Error("aero.frontWing.Available = false, want true")
	}
	if !meta.IsFreeSetting {
		t.Error("aero.frontWing.IsFreeSetting = false, want true")
	}
	if meta.Min != 0 {
		t.Errorf("aero.frontWing.Min = %v, want 0", meta.Min)
	}
	if meta.Max != 30 {
		t.Errorf("aero.frontWing.Max = %v, want 30", meta.Max)
	}

	// Verify aero.rearWing (isFreeSetting=false)
	metaRW, ok := got.ParamMeta["aero.rearWing"]
	if !ok {
		t.Fatal("ParamMeta missing key \"aero.rearWing\"")
	}
	if metaRW.IsFreeSetting {
		t.Error("aero.rearWing.IsFreeSetting = true, want false")
	}

	// Verify wheel metadata (wheels.0.pressure)
	metaWP, ok := got.ParamMeta["wheels.0.pressure"]
	if !ok {
		t.Fatal("ParamMeta missing key \"wheels.0.pressure\"")
	}
	if metaWP.Min != 20 || metaWP.Max != 40 {
		t.Errorf("wheels.0.pressure range = [%v, %v], want [20, 40]", metaWP.Min, metaWP.Max)
	}

	// Verify unavailable parameter
	metaBD, ok := got.ParamMeta["wheels.3.brakeDisc"]
	if !ok {
		t.Fatal("ParamMeta missing key \"wheels.3.brakeDisc\"")
	}
	if metaBD.Available {
		t.Error("wheels.3.brakeDisc.Available = true, want false")
	}
}

func TestParseCarSetup_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"invalid JSON", "{not json}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCarSetup(tt.input)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestParseCarSetup_RealFixtureRoundTripJSON(t *testing.T) {
	// Parse the fixture, serialize to JSON, deserialize back, and compare.
	// This proves the adapter output is JSON-stable.
	data, err := os.ReadFile("testdata/carsetup.json")
	if err != nil {
		t.Skip("testdata/carsetup.json not found")
	}

	got, err := ParseCarSetup(string(data))
	if err != nil {
		t.Fatalf("ParseCarSetup: %v", err)
	}

	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var roundTrip telemetry.SetupSheet
	if err := json.Unmarshal(b, &roundTrip); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	// Spot-check key fields survived
	if roundTrip.Aero != got.Aero {
		t.Errorf("Aero mismatch: got %+v, roundtrip %+v", got.Aero, roundTrip.Aero)
	}
	if roundTrip.Gearbox != got.Gearbox {
		t.Errorf("Gearbox mismatch")
	}
	if roundTrip.GearGraph.NumForwardGears != got.GearGraph.NumForwardGears {
		t.Errorf("GearGraph.NumForwardGears mismatch")
	}
	for i := range got.Wheels {
		if roundTrip.Wheels[i] != got.Wheels[i] {
			t.Errorf("Wheels[%d] mismatch", i)
		}
	}
}
