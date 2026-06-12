package lmu

import (
	"testing"
)

func TestTableNameMapping_KnownAliases(t *testing.T) {
	tests := []struct {
		lmuName    string
		lapaceName string
	}{
		// Aliases that match canonical import names
		{"TyresRimTemp", "Rim Temp"},
		{"TyresCarcassTemp", "Carcass Temp"},
		{"TyresRubberTemp", "TyresRubberTemp"},
		{"TyresTempCentre", "TyresTempCentre"},
		{"TyresTempLeft", "TyresTempLeft"},
		{"TyresTempRight", "TyresTempRight"},
		{"Brakes Temp", "Brakes Temp"},
		{"Susp Pos", "Suspension Deflection"},
		{"OverheatingState", "Overheating"},
		{"FuelMixtureMap", "FuelMixtureMap"},
		{"FrontFlapActivated", "Front Flap"},
		{"RearFlapActivated", "Rear Flap"},
		{"RearFlapLegalStatus", "Rear Flap Legal"},
		{"TyresPressure", "TyresPressure"},
		{"TyresWear", "Tyres Wear"},
		{"CloudDarkness", "CloudDarkness"},
		{"AntiStall Activated", "Anti Stall"},
		{"Tyres Wear", "Tyres Wear"},
		// Identity (no alias needed)
		{"Ground Speed", "Ground Speed"},
		{"Engine RPM", "Engine RPM"},
		{"Wheel Speed", "Wheel Speed"},
	}

	for _, tt := range tests {
		got := MapTableName(tt.lmuName)
		if got != tt.lapaceName {
			t.Errorf("MapTableName(%q) = %q, want %q", tt.lmuName, got, tt.lapaceName)
		}
	}
}

func TestTableNameMapping_UnknownPassThrough(t *testing.T) {
	name := MapTableName("SomeNewChannel2026")
	if name != "SomeNewChannel2026" {
		t.Errorf("MapTableName for unknown channel should pass through, got %q", name)
	}
}

func TestTableCategorization(t *testing.T) {
	tests := []struct {
		name       string
		wantScalar bool
		wantEvent  bool
		wantWheel  bool
	}{
		{"Ground Speed", true, false, false},
		{"Engine RPM", true, false, false},
		{"Throttle Pos", true, false, false},
		{"Lap", false, true, false},
		{"Gear", false, true, false},
		{"ABS", false, true, false},
		{"TC", false, true, false},
		{"Wheel Speed", false, false, true},
		{"Brakes Temp", false, false, true},
		{"TyresRimTemp", false, false, true},
	}

	for _, tt := range tests {
		category := CategorizeTable(tt.name)
		if tt.wantScalar && category != CategoryScalar {
			t.Errorf("CategorizeTable(%q) = %v, want scalar", tt.name, category)
		}
		if tt.wantEvent && category != CategoryEvent {
			t.Errorf("CategorizeTable(%q) = %v, want event", tt.name, category)
		}
		if tt.wantWheel && category != CategoryWheel {
			t.Errorf("CategorizeTable(%q) = %v, want wheel", tt.name, category)
		}
	}
}

func TestAllRealTablesCategorized(t *testing.T) {
	// Every table from the real LMU sample data must have a known category.
	// This validation catches tables we forgot to classify.
	uncategorized := []string{}
	for _, name := range realLMUTables() {
		if CategorizeTable(name) == CategoryUnknown {
			uncategorized = append(uncategorized, name)
		}
	}
	if len(uncategorized) > 0 {
		t.Errorf("These LMU tables are uncategorized: %v\nAdd them to the appropriate map in mapping.go", uncategorized)
	}
}

// realLMUTables lists all non-metadata table names from the actual LMU DuckDB sample data.
// This is the ground truth from sampledata/lmu_duckdb/.
func realLMUTables() []string {
	return []string{
		// Scalar channels (value FLOAT, no ts)
		"Ground Speed", "Engine RPM", "Throttle Pos", "Throttle Pos Unfiltered",
		"Brake Pos", "Brake Pos Unfiltered", "Clutch Pos", "Clutch Pos Unfiltered",
		"Clutch RPM", "Steering Pos", "Steering Pos Unfiltered", "Steering Shaft Torque",
		"FFB Output", "G Force Lat", "G Force Long", "G Force Vert",
		"GPS Latitude", "GPS Longitude", "GPS Speed", "GPS Time",
		"Fuel Level", "Engine Water Temp", "Engine Oil Temp", "Engine Max RPM",
		"Track Temperature", "Ambient Temperature", "Wind Speed", "Wind Heading",
		"Lap Dist", "Total Dist", "Path Lateral", "Track Edge",
		"Front3rdDeflection", "Rear3rdDeflection", "FrontRideHeight", "RearRideHeight",
		"Turbo Boost Pressure", "Virtual Energy", "SoC", "Regen Rate",
		"Time Behind Next", "OverheatingState",

		// Event channels (ts DOUBLE, value <type>)
		"Lap", "Gear", "ABS", "TC", "TCLevel", "TCCut", "TCSlipAngle", "ABSLevel",
		"Speed Limiter", "In Pits", "Current Sector", "Sector1 Flag", "Sector2 Flag",
		"Sector3 Flag", "Finish Status", "Headlights State", "Current LapTime",
		"Current Sector1", "Current Sector2", "Best LapTime", "Best Sector1",
		"Best Sector2", "Last Sector1", "Last Sector2", "Lap Time",
		"Brake Bias Rear", "Brake Migration", "Minimum Path Wetness",
		"CloudDarkness", "OffpathWetness", "FrontFlapActivated", "RearFlapActivated",
		"RearFlapLegalStatus", "AntiStall Activated", "LastImpactMagnitude",
		"LaunchControlActive", "FuelMixtureMap", "Yellow Flag State",

		// Wheel channels (value1-4 FLOAT, no ts)
		"Wheel Speed", "Brakes Temp", "Brakes Air Temp", "Brakes Force",
		"TyresRimTemp", "TyresCarcassTemp", "TyresRubberTemp", "TyresTempCentre",
		"TyresTempLeft", "TyresTempRight", "TyresPressure", "Tyres Wear",
		"Susp Pos", "RideHeights", "Brake Thickness",

		// Event-wheel channels (ts DOUBLE, value1-4 <type>)
		"SurfaceTypes", "WheelsDetached", "TyresCompound",
	}
}
