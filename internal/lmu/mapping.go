package lmu

// TableCategory classifies an LMU channel table by its column structure.
type TableCategory int

const (
	CategoryUnknown    TableCategory = iota
	CategoryScalar                   // value FLOAT (no ts) — e.g. Ground Speed, Engine RPM
	CategoryEvent                    // ts DOUBLE, value <type> — e.g. Lap, Gear, ABS
	CategoryWheel                    // value1-4 FLOAT (no ts) — e.g. Wheel Speed, Brakes Temp
	CategoryEventWheel               // ts DOUBLE, value1-4 <type> — e.g. SurfaceTypes, WheelsDetached
)

func (c TableCategory) String() string {
	switch c {
	case CategoryScalar:
		return "scalar"
	case CategoryEvent:
		return "event"
	case CategoryWheel:
		return "wheel"
	case CategoryEventWheel:
		return "event-wheel"
	default:
		return "unknown"
	}
}

// lmuNameAliases maps LMU table names to their canonical LaPace import names.
// Unknown names stay unchanged for source profiles, signal-family mapping, and longtail preservation.
var lmuNameAliases = map[string]string{
	// Canonical matches (loader looks up these names)
	"TyresRimTemp":        "Rim Temp",
	"TyresCarcassTemp":    "Carcass Temp",
	"TyresRubberTemp":     "TyresRubberTemp",
	"TyresTempCentre":     "TyresTempCentre",
	"TyresTempLeft":       "TyresTempLeft",
	"TyresTempRight":      "TyresTempRight",
	"Brakes Temp":         "Brakes Temp",
	"Susp Pos":            "Suspension Deflection",
	"OverheatingState":    "Overheating",
	"FuelMixtureMap":      "FuelMixtureMap",
	"FrontFlapActivated":  "Front Flap",
	"RearFlapActivated":   "Rear Flap",
	"RearFlapLegalStatus": "Rear Flap Legal",
	"TyresPressure":       "TyresPressure",
	"TyresWear":           "Tyres Wear",
	"Tyres Wear":          "Tyres Wear",
	"CloudDarkness":       "CloudDarkness",
	"AntiStall Activated": "Anti Stall",
	"LastImpactMagnitude": "Last Impact Magnitude",

	// Pass-through (not referenced by loader, keep as-is)
	"Brakes Air Temp":       "Brakes Air Temp",
	"Brakes Force":          "Brake Force",
	"Steering Shaft Torque": "Steering Shaft Torque",
	"OffpathWetness":        "Off Path Wetness",
	"TyresCompound":         "Tyre Compound",
	"WheelsDetached":        "Wheels Detached",
	"LaunchControlActive":   "Launch Control Active",
}

// MapTableName returns the Lapace-equivalent name for an LMU table name.
// Unknown names pass through unchanged.
func MapTableName(lmuName string) string {
	if alias, ok := lmuNameAliases[lmuName]; ok {
		return alias
	}
	return lmuName
}

// scalarChannels is the set of LMU tables with (value FLOAT) and no ts column.
var scalarChannels = map[string]bool{
	"Ground Speed":            true,
	"Engine RPM":              true,
	"Throttle Pos":            true,
	"Throttle Pos Unfiltered": true,
	"Brake Pos":               true,
	"Brake Pos Unfiltered":    true,
	"Clutch Pos":              true,
	"Clutch Pos Unfiltered":   true,
	"Clutch RPM":              true,
	"Steering Pos":            true,
	"Steering Pos Unfiltered": true,
	"Steering Shaft Torque":   true,
	"FFB Output":              true,
	"G Force Lat":             true,
	"G Force Long":            true,
	"G Force Vert":            true,
	"GPS Latitude":            true,
	"GPS Longitude":           true,
	"GPS Speed":               true,
	"GPS Time":                true,
	"Fuel Level":              true,
	"Engine Water Temp":       true,
	"Engine Oil Temp":         true,
	"Engine Max RPM":          true,
	"Track Temperature":       true,
	"Ambient Temperature":     true,
	"Wind Speed":              true,
	"Wind Heading":            true,
	"Lap Dist":                true,
	"Total Dist":              true,
	"Path Lateral":            true,
	"Track Edge":              true,
	"Lap Completion":          true,
	"Position X":              true,
	"Position Y":              true,
	"Position Z":              true,
	"Front3rdDeflection":      true,
	"Rear3rdDeflection":       true,
	"FrontRideHeight":         true,
	"RearRideHeight":          true,
	"Turbo Boost Pressure":    true,
	"Virtual Energy":          true,
	"SoC":                     true,
	"Regen Rate":              true,
	"Time Behind Next":        true,
	"OverheatingState":        true,
}

// eventChannels is the set of LMU tables with (ts DOUBLE, value <type>).
var eventChannels = map[string]bool{
	"Lap":                  true,
	"Position":             true,
	"Class Position":       true,
	"Gear":                 true,
	"ABS":                  true,
	"TC":                   true,
	"TCLevel":              true,
	"TCCut":                true,
	"TCSlipAngle":          true,
	"ABSLevel":             true,
	"Speed Limiter":        true,
	"In Pits":              true,
	"Current Sector":       true,
	"Sector1 Flag":         true,
	"Sector2 Flag":         true,
	"Sector3 Flag":         true,
	"Finish Status":        true,
	"Headlights State":     true,
	"Current LapTime":      true,
	"Current Lap Time":     true,
	"Current Sector1":      true,
	"Current Sector2":      true,
	"Best LapTime":         true,
	"Best Sector1":         true,
	"Best Sector2":         true,
	"Last Sector1":         true,
	"Last Sector2":         true,
	"Last LapTime":         true,
	"Lap Time":             true,
	"Delta Best":           true,
	"Delta To Best":        true,
	"Delta to Best":        true,
	"Brake Bias Rear":      true,
	"Brake Migration":      true,
	"Minimum Path Wetness": true,
	"CloudDarkness":        true,
	"OffpathWetness":       true,
	"FrontFlapActivated":   true,
	"RearFlapActivated":    true,
	"RearFlapLegalStatus":  true,
	"AntiStall Activated":  true,
	"LastImpactMagnitude":  true,
	"LaunchControlActive":  true,
	"FuelMixtureMap":       true,
	"Yellow Flag State":    true,
}

// wheelChannels is the set of LMU tables with (value1-4 FLOAT) and no ts column.
var wheelChannels = map[string]bool{
	"Wheel Speed":      true,
	"Brakes Temp":      true,
	"Brakes Air Temp":  true,
	"Brakes Force":     true,
	"TyresRimTemp":     true,
	"TyresCarcassTemp": true,
	"TyresRubberTemp":  true,
	"TyresTempCentre":  true,
	"TyresTempLeft":    true,
	"TyresTempRight":   true,
	"TyresPressure":    true,
	"TyresWear":        true,
	"Tyres Wear":       true, // variant with space seen in some LMU exports
	"Susp Pos":         true,
	"RideHeights":      true,
	"Brake Thickness":  true,
}

// eventWheelChannels is the set of LMU tables with (ts DOUBLE, value1-4 <type>).
var eventWheelChannels = map[string]bool{
	"SurfaceTypes":   true,
	"WheelsDetached": true,
	"TyresCompound":  true,
}

// CategorizeTable determines the column structure of an LMU channel table.
func CategorizeTable(name string) TableCategory {
	if scalarChannels[name] {
		return CategoryScalar
	}
	if eventChannels[name] {
		return CategoryEvent
	}
	if wheelChannels[name] {
		return CategoryWheel
	}
	if eventWheelChannels[name] {
		return CategoryEventWheel
	}
	return CategoryUnknown
}
