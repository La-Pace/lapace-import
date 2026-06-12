package lmu

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/La-Pace/lapace-core/contract/telemetry"
)

// ParseCarSetup parses a raw LMU CarSetup JSON blob into a typed SetupSheet.
// The input is the JSON string read from the LMU DuckDB metadata table under
// the "CarSetup" key (~172 parameters organized as VM_, WM_, gearGraph, symmetric).
func ParseCarSetup(rawJSON string) (telemetry.SetupSheet, error) {
	if strings.TrimSpace(rawJSON) == "" {
		return telemetry.SetupSheet{}, fmt.Errorf("parse car setup: empty input")
	}

	// Parse into raw map first — each parameter is a JSON object with value/metadata.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
		return telemetry.SetupSheet{}, fmt.Errorf("parse car setup: %w", err)
	}

	var sheet telemetry.SetupSheet
	meta := make(map[string]telemetry.SetupParamMeta)

	// Parse special keys first
	if v, ok := raw["symmetric"]; ok {
		_ = json.Unmarshal(v, &sheet.Symmetric)
	}
	if v, ok := raw["gearGraph"]; ok {
		_ = json.Unmarshal(v, &sheet.GearGraph)
	}

	// Parse VM_ (Vehicle Model) keys — 110 parameters
	for key, rawVal := range raw {
		if !strings.HasPrefix(key, "VM_") {
			continue
		}
		param, err := parseLMUParam(rawVal)
		if err != nil {
			continue // skip malformed parameters
		}
		setVMField(key, param.Value, &sheet)

		// Build dot-path for ParamMeta
		if dotPath, ok := vmDotPath(key); ok {
			meta[dotPath] = telemetry.SetupParamMeta{
				Available:     param.Available,
				IsFreeSetting: param.IsFreeSetting,
				Min:           param.MinValue,
				Max:           param.MaxValue,
			}
		}
	}

	// Parse WM_ (Wheel Model) keys — 60 parameters (15 per wheel)
	for key, rawVal := range raw {
		if !strings.HasPrefix(key, "WM_") {
			continue
		}
		param, err := parseLMUParam(rawVal)
		if err != nil {
			continue
		}

		wheelIdx, fieldName, dotPath, ok := parseWMKey(key)
		if !ok {
			continue
		}
		setWheelField(&sheet, wheelIdx, fieldName, param.Value)
		meta[dotPath] = telemetry.SetupParamMeta{
			Available:     param.Available,
			IsFreeSetting: param.IsFreeSetting,
			Min:           param.MinValue,
			Max:           param.MaxValue,
		}
	}

	if len(meta) > 0 {
		sheet.ParamMeta = meta
	}

	return sheet, nil
}

// lmuParam is the internal structure of each VM_/WM_ parameter in the LMU JSON.
type lmuParam struct {
	Available         bool    `json:"available"`
	IsFreeSetting     bool    `json:"isFreeSetting"`
	MinValue          float64 `json:"minValue"`
	MaxValue          float64 `json:"maxValue"`
	Value             float64 `json:"value"`
	StringValue       string  `json:"stringValue"`
	LastSavedStrValue string  `json:"lastSavedStringValue"`
	NumChanges        float64 `json:"numChangesValue"`
	DiffComparison    float64 `json:"diffComparisonValue"`
}

func parseLMUParam(raw json.RawMessage) (lmuParam, error) {
	var p lmuParam
	if err := json.Unmarshal(raw, &p); err != nil {
		return lmuParam{}, err
	}
	return p, nil
}

// setVMField maps a VM_ key to the corresponding SetupSheet field.
func setVMField(key string, value float64, sheet *telemetry.SetupSheet) {
	switch key {
	// Aero
	case "VM_FRONT_WING":
		sheet.Aero.FrontWing = value
	case "VM_REAR_WING":
		sheet.Aero.RearWing = value

	// Front suspension
	case "VM_FRONT_3RD_FASTBUMP":
		sheet.Suspension.Front.ThirdSpringFastBump = value
	case "VM_FRONT_3RD_FASTREBOUND":
		sheet.Suspension.Front.ThirdSpringFastRebound = value
	case "VM_FRONT_3RD_PACKERS":
		sheet.Suspension.Front.ThirdSpringPackers = value
	case "VM_FRONT_3RD_SLOWBUMP":
		sheet.Suspension.Front.ThirdSpringSlowBump = value
	case "VM_FRONT_3RD_SLOWREBOUND":
		sheet.Suspension.Front.ThirdSpringSlowRebound = value
	case "VM_FRONT_3RD_SPRING":
		sheet.Suspension.Front.ThirdSpring = value
	case "VM_FRONT_3RD_TENDERSPRING":
		sheet.Suspension.Front.TenderSpring = value
	case "VM_FRONT_3RD_TENDERSPRINGTRAVEL":
		sheet.Suspension.Front.TenderSpringTravel = value
	case "VM_FRONT_ANTISWAY":
		sheet.Suspension.Front.AntiSway = value
	case "VM_FRONT_TIRE_COMPOUND":
		sheet.Suspension.Front.TireCompound = value
	case "VM_FRONT_TOEIN":
		sheet.Suspension.Front.ToeIn = value
	case "VM_FRONT_TOEOFFSET":
		sheet.Suspension.Front.ToeOffset = value
	case "VM_FRONT_WHEEL_TRACK":
		sheet.Suspension.Front.WheelTrack = value

	// Rear suspension
	case "VM_REAR_3RD_FASTBUMP":
		sheet.Suspension.Rear.ThirdSpringFastBump = value
	case "VM_REAR_3RD_FASTREBOUND":
		sheet.Suspension.Rear.ThirdSpringFastRebound = value
	case "VM_REAR_3RD_PACKERS":
		sheet.Suspension.Rear.ThirdSpringPackers = value
	case "VM_REAR_3RD_SLOWBUMP":
		sheet.Suspension.Rear.ThirdSpringSlowBump = value
	case "VM_REAR_3RD_SLOWREBOUND":
		sheet.Suspension.Rear.ThirdSpringSlowRebound = value
	case "VM_REAR_3RD_SPRING":
		sheet.Suspension.Rear.ThirdSpring = value
	case "VM_REAR_3RD_TENDERSPRING":
		sheet.Suspension.Rear.TenderSpring = value
	case "VM_REAR_3RD_TENDERSPRINGTRAVEL":
		sheet.Suspension.Rear.TenderSpringTravel = value
	case "VM_REAR_ANTISWAY":
		sheet.Suspension.Rear.AntiSway = value
	case "VM_REAR_TIRE_COMPOUND":
		sheet.Suspension.Rear.TireCompound = value
	case "VM_REAR_TOEIN":
		sheet.Suspension.Rear.ToeIn = value
	case "VM_REAR_TOEOFFSET":
		sheet.Suspension.Rear.ToeOffset = value
	case "VM_REAR_WHEEL_TRACK":
		sheet.Suspension.Rear.WheelTrack = value

	// Differential
	case "VM_DIFF_COAST":
		sheet.Differential.Coast = value
	case "VM_DIFF_POWER":
		sheet.Differential.Power = value
	case "VM_DIFF_PRELOAD":
		sheet.Differential.Preload = value
	case "VM_DIFF_PUMP":
		sheet.Differential.Pump = value
	case "VM_FRONT_DIFF_COAST":
		sheet.Differential.FrontCoast = value
	case "VM_FRONT_DIFF_POWER":
		sheet.Differential.FrontPower = value
	case "VM_FRONT_DIFF_PRELOAD":
		sheet.Differential.FrontPreload = value
	case "VM_FRONT_DIFF_PUMP":
		sheet.Differential.FrontPump = value

	// Brakes
	case "VM_ANTILOCKBRAKESYSTEMMAP":
		sheet.Brakes.ABSMap = value
	case "VM_ANTILOCK_BRAKES":
		sheet.Brakes.ABS = value
	case "VM_BRAKE_BALANCE":
		sheet.Brakes.Balance = value
	case "VM_BRAKE_DUCTS":
		sheet.Brakes.FrontDucts = value
	case "VM_BRAKE_DUCTS_REAR":
		sheet.Brakes.RearDucts = value
	case "VM_BRAKE_MIGRATION":
		sheet.Brakes.Migration = value
	case "VM_BRAKE_PRESSURE":
		sheet.Brakes.Pressure = value
	case "VM_HANDBRAKE_PRESSURE":
		sheet.Brakes.HandbrakePressure = value
	case "VM_HANDFRONTBRAKE_PRESSURE":
		sheet.Brakes.HandFrontBrakePressure = value

	// Engine
	case "VM_ENGINE_BOOST":
		sheet.Engine.Boost = value
	case "VM_ENGINE_BRAKEMAP":
		sheet.Engine.BrakeMap = value
	case "VM_ENGINE_MIXTURE":
		sheet.Engine.Mixture = value
	case "VM_REV_LIMITER":
		sheet.Engine.RevLimiter = value

	// Traction control
	case "VM_TRACTION_CONTROL":
		sheet.TractionControl.Level = value
	case "VM_TRACTIONCONTROLMAP":
		sheet.TractionControl.Map = value
	case "VM_TRACTIONCONTROLPOWERCUTMAP":
		sheet.TractionControl.PowerCutMap = value
	case "VM_TRACTIONCONTROLSLIPANGLEMAP":
		sheet.TractionControl.SlipAngleMap = value

	// Gearbox
	case "VM_GEAR_1":
		sheet.Gearbox.Gear1 = value
	case "VM_GEAR_2":
		sheet.Gearbox.Gear2 = value
	case "VM_GEAR_3":
		sheet.Gearbox.Gear3 = value
	case "VM_GEAR_4":
		sheet.Gearbox.Gear4 = value
	case "VM_GEAR_5":
		sheet.Gearbox.Gear5 = value
	case "VM_GEAR_6":
		sheet.Gearbox.Gear6 = value
	case "VM_GEAR_7":
		sheet.Gearbox.Gear7 = value
	case "VM_GEAR_8":
		sheet.Gearbox.Gear8 = value
	case "VM_GEAR_9":
		sheet.Gearbox.Gear9 = value
	case "VM_GEAR_FINAL":
		sheet.Gearbox.FinalDrive = value
	case "VM_GEAR_REVERSE":
		sheet.Gearbox.ReverseGear = value
	case "VM_GEAR_AUTODOWNSHIFT":
		sheet.Gearbox.AutoDownshift = value
	case "VM_GEAR_AUTOUPSHIFT":
		sheet.Gearbox.AutoUpshift = value
	case "VM_GEAR_UPSHIFT_RPM_1":
		sheet.Gearbox.UpshiftRPM1 = value
	case "VM_GEAR_UPSHIFT_RPM_2":
		sheet.Gearbox.UpshiftRPM2 = value
	case "VM_GEAR_UPSHIFT_RPM_3":
		sheet.Gearbox.UpshiftRPM3 = value
	case "VM_GEAR_UPSHIFT_RPM_4":
		sheet.Gearbox.UpshiftRPM4 = value
	case "VM_GEAR_UPSHIFT_RPM_5":
		sheet.Gearbox.UpshiftRPM5 = value
	case "VM_GEAR_UPSHIFT_RPM_6":
		sheet.Gearbox.UpshiftRPM6 = value
	case "VM_GEAR_UPSHIFT_RPM_7":
		sheet.Gearbox.UpshiftRPM7 = value
	case "VM_GEAR_UPSHIFT_RPM_8":
		sheet.Gearbox.UpshiftRPM8 = value
	case "VM_RATIO_SET":
		sheet.Gearbox.RatioSet = value

	// Chassis
	case "VM_CHASSIS_ADJ_00":
		sheet.Chassis.Adjustments[0] = value
	case "VM_CHASSIS_ADJ_01":
		sheet.Chassis.Adjustments[1] = value
	case "VM_CHASSIS_ADJ_02":
		sheet.Chassis.Adjustments[2] = value
	case "VM_CHASSIS_ADJ_03":
		sheet.Chassis.Adjustments[3] = value
	case "VM_CHASSIS_ADJ_04":
		sheet.Chassis.Adjustments[4] = value
	case "VM_CHASSIS_ADJ_05":
		sheet.Chassis.Adjustments[5] = value
	case "VM_CHASSIS_ADJ_06":
		sheet.Chassis.Adjustments[6] = value
	case "VM_CHASSIS_ADJ_07":
		sheet.Chassis.Adjustments[7] = value
	case "VM_CHASSIS_ADJ_08":
		sheet.Chassis.Adjustments[8] = value
	case "VM_CHASSIS_ADJ_09":
		sheet.Chassis.Adjustments[9] = value
	case "VM_CHASSIS_ADJ_10":
		sheet.Chassis.Adjustments[10] = value
	case "VM_CHASSIS_ADJ_11":
		sheet.Chassis.Adjustments[11] = value
	case "VM_LEFT_CASTER":
		sheet.Chassis.LeftCaster = value
	case "VM_RIGHT_CASTER":
		sheet.Chassis.RightCaster = value
	case "VM_LEFT_FENDER_FLARE":
		sheet.Chassis.LeftFenderFlare = value
	case "VM_RIGHT_FENDER_FLARE":
		sheet.Chassis.RightFenderFlare = value
	case "VM_LEFT_TRACK_BAR":
		sheet.Chassis.LeftTrackBar = value
	case "VM_RIGHT_TRACK_BAR":
		sheet.Chassis.RightTrackBar = value
	case "VM_STEER_LOCK":
		sheet.Chassis.SteerLock = value
	case "VM_TORQUE_SPLIT":
		sheet.Chassis.TorqueSplit = value

	// Weight
	case "VM_WEIGHT_DISTRIB":
		sheet.Weight.Distribution = value
	case "VM_WEIGHT_LATERAL":
		sheet.Weight.Lateral = value
	case "VM_WEIGHT_VERTICAL":
		sheet.Weight.Vertical = value
	case "VM_WEIGHT_WEDGE":
		sheet.Weight.Wedge = value

	// Cooling
	case "VM_OIL_RADIATOR":
		sheet.Cooling.OilRadiator = value
	case "VM_WATER_RADIATOR":
		sheet.Cooling.WaterRadiator = value

	// Fuel
	case "VM_FUEL_CAPACITY":
		sheet.Fuel.Capacity = value
	case "VM_FUEL_LEVEL":
		sheet.Fuel.Level = value
	case "VM_REGEN_LEVEL":
		sheet.Fuel.RegenLevel = value
	case "VM_VIRTUAL_ENERGY":
		sheet.Fuel.VirtualEnergy = value

	// Strategy
	case "VM_NUM_PITSTOPS":
		sheet.Strategy.NumPitStops = value
	case "VM_PITSTOP_1":
		sheet.Strategy.PitStop1 = value
	case "VM_PITSTOP_2":
		sheet.Strategy.PitStop2 = value
	case "VM_PITSTOP_3":
		sheet.Strategy.PitStop3 = value

	// ERS
	case "VM_ELECTRIC_MOTOR_MAP":
		sheet.ERS.ElectricMotorMap = value
	}
}

// vmDotPath returns the JSON dot-path for a VM_ key for use in ParamMeta.
func vmDotPath(key string) (string, bool) {
	paths := map[string]string{
		"VM_FRONT_WING": "aero.frontWing",
		"VM_REAR_WING":  "aero.rearWing",

		"VM_FRONT_3RD_FASTBUMP":           "suspension.front.thirdSpringFastBump",
		"VM_FRONT_3RD_FASTREBOUND":        "suspension.front.thirdSpringFastRebound",
		"VM_FRONT_3RD_PACKERS":            "suspension.front.thirdSpringPackers",
		"VM_FRONT_3RD_SLOWBUMP":           "suspension.front.thirdSpringSlowBump",
		"VM_FRONT_3RD_SLOWREBOUND":        "suspension.front.thirdSpringSlowRebound",
		"VM_FRONT_3RD_SPRING":             "suspension.front.thirdSpring",
		"VM_FRONT_3RD_TENDERSPRING":       "suspension.front.tenderSpring",
		"VM_FRONT_3RD_TENDERSPRINGTRAVEL": "suspension.front.tenderSpringTravel",
		"VM_FRONT_ANTISWAY":               "suspension.front.antiSway",
		"VM_FRONT_TIRE_COMPOUND":          "suspension.front.tireCompound",
		"VM_FRONT_TOEIN":                  "suspension.front.toeIn",
		"VM_FRONT_TOEOFFSET":              "suspension.front.toeOffset",
		"VM_FRONT_WHEEL_TRACK":            "suspension.front.wheelTrack",

		"VM_REAR_3RD_FASTBUMP":           "suspension.rear.thirdSpringFastBump",
		"VM_REAR_3RD_FASTREBOUND":        "suspension.rear.thirdSpringFastRebound",
		"VM_REAR_3RD_PACKERS":            "suspension.rear.thirdSpringPackers",
		"VM_REAR_3RD_SLOWBUMP":           "suspension.rear.thirdSpringSlowBump",
		"VM_REAR_3RD_SLOWREBOUND":        "suspension.rear.thirdSpringSlowRebound",
		"VM_REAR_3RD_SPRING":             "suspension.rear.thirdSpring",
		"VM_REAR_3RD_TENDERSPRING":       "suspension.rear.tenderSpring",
		"VM_REAR_3RD_TENDERSPRINGTRAVEL": "suspension.rear.tenderSpringTravel",
		"VM_REAR_ANTISWAY":               "suspension.rear.antiSway",
		"VM_REAR_TIRE_COMPOUND":          "suspension.rear.tireCompound",
		"VM_REAR_TOEIN":                  "suspension.rear.toeIn",
		"VM_REAR_TOEOFFSET":              "suspension.rear.toeOffset",
		"VM_REAR_WHEEL_TRACK":            "suspension.rear.wheelTrack",

		"VM_DIFF_COAST":         "differential.coast",
		"VM_DIFF_POWER":         "differential.power",
		"VM_DIFF_PRELOAD":       "differential.preload",
		"VM_DIFF_PUMP":          "differential.pump",
		"VM_FRONT_DIFF_COAST":   "differential.frontCoast",
		"VM_FRONT_DIFF_POWER":   "differential.frontPower",
		"VM_FRONT_DIFF_PRELOAD": "differential.frontPreload",
		"VM_FRONT_DIFF_PUMP":    "differential.frontPump",

		"VM_ANTILOCKBRAKESYSTEMMAP":  "brakes.absMap",
		"VM_ANTILOCK_BRAKES":         "brakes.abs",
		"VM_BRAKE_BALANCE":           "brakes.balance",
		"VM_BRAKE_DUCTS":             "brakes.frontDucts",
		"VM_BRAKE_DUCTS_REAR":        "brakes.rearDucts",
		"VM_BRAKE_MIGRATION":         "brakes.migration",
		"VM_BRAKE_PRESSURE":          "brakes.pressure",
		"VM_HANDBRAKE_PRESSURE":      "brakes.handbrakePressure",
		"VM_HANDFRONTBRAKE_PRESSURE": "brakes.handFrontBrakePressure",

		"VM_ENGINE_BOOST":    "engine.boost",
		"VM_ENGINE_BRAKEMAP": "engine.brakeMap",
		"VM_ENGINE_MIXTURE":  "engine.mixture",
		"VM_REV_LIMITER":     "engine.revLimiter",

		"VM_TRACTION_CONTROL":            "tractionControl.level",
		"VM_TRACTIONCONTROLMAP":          "tractionControl.map",
		"VM_TRACTIONCONTROLPOWERCUTMAP":  "tractionControl.powerCutMap",
		"VM_TRACTIONCONTROLSLIPANGLEMAP": "tractionControl.slipAngleMap",

		"VM_GEAR_1":             "gearbox.gear1",
		"VM_GEAR_2":             "gearbox.gear2",
		"VM_GEAR_3":             "gearbox.gear3",
		"VM_GEAR_4":             "gearbox.gear4",
		"VM_GEAR_5":             "gearbox.gear5",
		"VM_GEAR_6":             "gearbox.gear6",
		"VM_GEAR_7":             "gearbox.gear7",
		"VM_GEAR_8":             "gearbox.gear8",
		"VM_GEAR_9":             "gearbox.gear9",
		"VM_GEAR_FINAL":         "gearbox.finalDrive",
		"VM_GEAR_REVERSE":       "gearbox.reverseGear",
		"VM_GEAR_AUTODOWNSHIFT": "gearbox.autoDownshift",
		"VM_GEAR_AUTOUPSHIFT":   "gearbox.autoUpshift",
		"VM_GEAR_UPSHIFT_RPM_1": "gearbox.upshiftRpm1",
		"VM_GEAR_UPSHIFT_RPM_2": "gearbox.upshiftRpm2",
		"VM_GEAR_UPSHIFT_RPM_3": "gearbox.upshiftRpm3",
		"VM_GEAR_UPSHIFT_RPM_4": "gearbox.upshiftRpm4",
		"VM_GEAR_UPSHIFT_RPM_5": "gearbox.upshiftRpm5",
		"VM_GEAR_UPSHIFT_RPM_6": "gearbox.upshiftRpm6",
		"VM_GEAR_UPSHIFT_RPM_7": "gearbox.upshiftRpm7",
		"VM_GEAR_UPSHIFT_RPM_8": "gearbox.upshiftRpm8",
		"VM_RATIO_SET":          "gearbox.ratioSet",

		"VM_CHASSIS_ADJ_00":     "chassis.adjustments.0",
		"VM_CHASSIS_ADJ_01":     "chassis.adjustments.1",
		"VM_CHASSIS_ADJ_02":     "chassis.adjustments.2",
		"VM_CHASSIS_ADJ_03":     "chassis.adjustments.3",
		"VM_CHASSIS_ADJ_04":     "chassis.adjustments.4",
		"VM_CHASSIS_ADJ_05":     "chassis.adjustments.5",
		"VM_CHASSIS_ADJ_06":     "chassis.adjustments.6",
		"VM_CHASSIS_ADJ_07":     "chassis.adjustments.7",
		"VM_CHASSIS_ADJ_08":     "chassis.adjustments.8",
		"VM_CHASSIS_ADJ_09":     "chassis.adjustments.9",
		"VM_CHASSIS_ADJ_10":     "chassis.adjustments.10",
		"VM_CHASSIS_ADJ_11":     "chassis.adjustments.11",
		"VM_LEFT_CASTER":        "chassis.leftCaster",
		"VM_RIGHT_CASTER":       "chassis.rightCaster",
		"VM_LEFT_FENDER_FLARE":  "chassis.leftFenderFlare",
		"VM_RIGHT_FENDER_FLARE": "chassis.rightFenderFlare",
		"VM_LEFT_TRACK_BAR":     "chassis.leftTrackBar",
		"VM_RIGHT_TRACK_BAR":    "chassis.rightTrackBar",
		"VM_STEER_LOCK":         "chassis.steerLock",
		"VM_TORQUE_SPLIT":       "chassis.torqueSplit",

		"VM_WEIGHT_DISTRIB":  "weight.distribution",
		"VM_WEIGHT_LATERAL":  "weight.lateral",
		"VM_WEIGHT_VERTICAL": "weight.vertical",
		"VM_WEIGHT_WEDGE":    "weight.wedge",

		"VM_OIL_RADIATOR":   "cooling.oilRadiator",
		"VM_WATER_RADIATOR": "cooling.waterRadiator",

		"VM_FUEL_CAPACITY":  "fuel.capacity",
		"VM_FUEL_LEVEL":     "fuel.level",
		"VM_REGEN_LEVEL":    "fuel.regenLevel",
		"VM_VIRTUAL_ENERGY": "fuel.virtualEnergy",

		"VM_NUM_PITSTOPS": "strategy.numPitStops",
		"VM_PITSTOP_1":    "strategy.pitStop1",
		"VM_PITSTOP_2":    "strategy.pitStop2",
		"VM_PITSTOP_3":    "strategy.pitStop3",

		"VM_ELECTRIC_MOTOR_MAP": "ers.electricMotorMap",
	}

	p, ok := paths[key]
	return p, ok
}

// wheelSuffix maps LMU wheel suffix to array index.
var wheelSuffix = map[string]int{
	"W_FL": 0,
	"W_FR": 1,
	"W_RL": 2,
	"W_RR": 3,
}

// wmParamName maps LMU WM_ param names to WheelSetup field names and JSON keys.
var wmParamName = map[string]struct {
	fieldName string
	jsonKey   string
}{
	"BRAKEDISC":          {fieldName: "BrakeDisc", jsonKey: "brakeDisc"},
	"BRAKEPAD":           {fieldName: "BrakePad", jsonKey: "brakePad"},
	"CAMBER":             {fieldName: "Camber", jsonKey: "camber"},
	"COMPOUND":           {fieldName: "Compound", jsonKey: "compound"},
	"FASTBUMP":           {fieldName: "FastBump", jsonKey: "fastBump"},
	"FASTREBOUND":        {fieldName: "FastRebound", jsonKey: "fastRebound"},
	"PACKERS":            {fieldName: "Packers", jsonKey: "packers"},
	"PRESSURE":           {fieldName: "Pressure", jsonKey: "pressure"},
	"RIDEHEIGHT":         {fieldName: "RideHeight", jsonKey: "rideHeight"},
	"SLOWBUMP":           {fieldName: "SlowBump", jsonKey: "slowBump"},
	"SLOWREBOUND":        {fieldName: "SlowRebound", jsonKey: "slowRebound"},
	"SPRING":             {fieldName: "Spring", jsonKey: "spring"},
	"SRUBBER":            {fieldName: "ScrubRubber", jsonKey: "scrubRubber"},
	"TENDERSPRING":       {fieldName: "TenderSpring", jsonKey: "tenderSpring"},
	"TENDERSPRINGTRAVEL": {fieldName: "TenderSpringTravel", jsonKey: "tenderSpringTravel"},
}

// parseWMKey parses a WM_ key into (wheelIndex, fieldName, dotPath, ok).
// WM_BRAKEDISC-W_FL → (0, "BrakeDisc", "wheels.0.brakeDisc", true)
func parseWMKey(key string) (int, string, string, bool) {
	// Key format: WM_<PARAM>-W_<SUFFIX>
	if !strings.HasPrefix(key, "WM_") {
		return 0, "", "", false
	}

	// Find the last "-W_" which separates param name from wheel suffix
	idx := strings.LastIndex(key, "-W_")
	if idx < 0 {
		return 0, "", "", false
	}

	paramPart := key[3:idx]   // e.g., "BRAKEDISC"
	suffixPart := key[idx+1:] // e.g., "W_FL"

	wheelIdx, ok := wheelSuffix[suffixPart]
	if !ok {
		return 0, "", "", false
	}

	param, ok := wmParamName[paramPart]
	if !ok {
		return 0, "", "", false
	}

	dotPath := fmt.Sprintf("wheels.%d.%s", wheelIdx, param.jsonKey)
	return wheelIdx, param.fieldName, dotPath, true
}

// setWheelField sets a specific field on a specific wheel using reflection-free
// direct assignment. Faster than reflect and the set of fields is small and fixed.
func setWheelField(sheet *telemetry.SetupSheet, wheelIdx int, fieldName string, value float64) {
	w := &sheet.Wheels[wheelIdx]
	switch fieldName {
	case "BrakeDisc":
		w.BrakeDisc = value
	case "BrakePad":
		w.BrakePad = value
	case "Camber":
		w.Camber = value
	case "Compound":
		w.Compound = value
	case "FastBump":
		w.FastBump = value
	case "FastRebound":
		w.FastRebound = value
	case "Packers":
		w.Packers = value
	case "Pressure":
		w.Pressure = value
	case "RideHeight":
		w.RideHeight = value
	case "SlowBump":
		w.SlowBump = value
	case "SlowRebound":
		w.SlowRebound = value
	case "Spring":
		w.Spring = value
	case "ScrubRubber":
		w.ScrubRubber = value
	case "TenderSpring":
		w.TenderSpring = value
	case "TenderSpringTravel":
		w.TenderSpringTravel = value
	}
}
