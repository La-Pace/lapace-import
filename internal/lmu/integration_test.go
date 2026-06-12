package lmu

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/La-Pace/lapace-import/internal/core"
	schema "github.com/La-Pace/lapace-import/internal/schema"
	_ "github.com/duckdb/duckdb-go/v2"
)

func TestImportAllWritesSignalFamilyCoreTables(t *testing.T) {
	inputPath := createMinimalLMUExport(t)

	lmu, err := OpenLMUFile(inputPath)
	if err != nil {
		t.Fatalf("OpenLMUFile error: %v", err)
	}
	defer lmu.Close()

	outputPath := filepath.Join(t.TempDir(), "signal-family.duckdb")
	writer, err := core.NewWriter(outputPath)
	if err != nil {
		t.Fatalf("core.NewWriter error: %v", err)
	}

	stats, err := ImportAll(context.Background(), lmu, writer)
	if err != nil {
		t.Fatalf("ImportAll error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Writer Close error: %v", err)
	}
	if stats.TablesProcessed == 0 {
		t.Fatal("expected importer to process LMU tables")
	}

	db, err := sql.Open("duckdb", outputPath+"?access_mode=READ_ONLY")
	if err != nil {
		t.Fatalf("open output DB: %v", err)
	}
	defer db.Close()

	var schemaType string
	if err := db.QueryRow(`SELECT schema_type FROM lapace_version LIMIT 1`).Scan(&schemaType); err != nil {
		t.Fatalf("read schema_type: %v", err)
	}
	if schemaType != schema.SignalFamilySchemaType {
		t.Fatalf("schema_type = %q, want %q", schemaType, schema.SignalFamilySchemaType)
	}

	for _, table := range schema.SignalFamilyTableNames {
		if !testTableExists(t, db, table) {
			t.Fatalf("signal-family table %q was not created", table)
		}
	}
	if testTableExists(t, db, "Ground Speed") {
		t.Fatal("legacy narrow table Ground Speed should not be written")
	}

	assertTableCount(t, db, "source_samples", 4)
	for _, table := range []string{
		"driver_controls",
		"vehicle_dynamics",
		"powertrain",
		"progress_position",
		"race_standing",
		"lap_reference_timing",
		"current_lap_timing",
	} {
		assertTableCount(t, db, table, 4)
	}

	var minSeq, maxSeq, distinctSeq int64
	if err := db.QueryRow(`SELECT MIN(sample_seq), MAX(sample_seq), COUNT(DISTINCT sample_seq) FROM source_samples`).Scan(&minSeq, &maxSeq, &distinctSeq); err != nil {
		t.Fatalf("read source sample spine: %v", err)
	}
	if minSeq != 0 || maxSeq != 3 || distinctSeq != 4 {
		t.Fatalf("source sample spine = min %d max %d distinct %d, want dense 0..3", minSeq, maxSeq, distinctSeq)
	}

	var source string
	var deltaTime, currentLapTime float64
	if err := db.QueryRow(`SELECT source, delta_time, current_lap_time FROM source_samples WHERE sample_seq = 1`).Scan(&source, &deltaTime, &currentLapTime); err != nil {
		t.Fatalf("read source_samples seq 1: %v", err)
	}
	if source != "lmu-official-import" {
		t.Fatalf("source = %q, want lmu-official-import", source)
	}
	if deltaTime != 0.25 {
		t.Fatalf("delta_time = %v, want 0.25", deltaTime)
	}
	if currentLapTime != 0.25 {
		t.Fatalf("current_lap_time = %v, want 0.25", currentLapTime)
	}

	var brakeBefore, brakeAfter float64
	if err := db.QueryRow(`SELECT brake_pos FROM driver_controls WHERE sample_seq = 1`).Scan(&brakeBefore); err != nil {
		t.Fatalf("read brake before LOCF change: %v", err)
	}
	if err := db.QueryRow(`SELECT brake_pos FROM driver_controls WHERE sample_seq = 2`).Scan(&brakeAfter); err != nil {
		t.Fatalf("read brake after LOCF change: %v", err)
	}
	if brakeBefore != 0.0 || brakeAfter != 0.4 {
		t.Fatalf("brake LOCF values = before %v after %v, want 0.0 then 0.4", brakeBefore, brakeAfter)
	}

	var lap, currentSector, position, classPosition int64
	if err := db.QueryRow(`SELECT lap, current_sector FROM progress_position WHERE sample_seq = 2`).Scan(&lap, &currentSector); err != nil {
		t.Fatalf("read progress_position seq 2: %v", err)
	}
	if lap != 1 || currentSector != 2 {
		t.Fatalf("progress_position seq 2 = lap %d sector %d, want lap 1 sector 2", lap, currentSector)
	}
	if err := db.QueryRow(`SELECT position, class_position FROM race_standing WHERE sample_seq = 2`).Scan(&position, &classPosition); err != nil {
		t.Fatalf("read race_standing seq 2: %v", err)
	}
	if position != 5 || classPosition != 2 {
		t.Fatalf("race_standing seq 2 = position %d class %d, want 5 and 2", position, classPosition)
	}

	var bestLap, deltaToBest, currentSector2 float64
	if err := db.QueryRow(`SELECT best_laptime FROM lap_reference_timing WHERE sample_seq = 2`).Scan(&bestLap); err != nil {
		t.Fatalf("read lap_reference_timing seq 2: %v", err)
	}
	if err := db.QueryRow(`SELECT delta_to_best_lap, current_sector2 FROM current_lap_timing WHERE sample_seq = 2`).Scan(&deltaToBest, &currentSector2); err != nil {
		t.Fatalf("read current_lap_timing seq 2: %v", err)
	}
	if bestLap != 80.1 || deltaToBest != -0.2 || currentSector2 != 54.8 {
		t.Fatalf("timing seq 2 = best %v delta %v sector2 %v, want 80.1, -0.2, 54.8", bestLap, deltaToBest, currentSector2)
	}

	var gear int64
	var brake, speed float64
	if err := db.QueryRow(`SELECT gear, brake_pos FROM driver_controls WHERE sample_seq = 2`).Scan(&gear, &brake); err != nil {
		t.Fatalf("read driver_controls seq 2: %v", err)
	}
	if err := db.QueryRow(`SELECT ground_speed FROM vehicle_dynamics WHERE sample_seq = 2`).Scan(&speed); err != nil {
		t.Fatalf("read vehicle_dynamics seq 2: %v", err)
	}
	if gear != 3 || brake != 0.4 || speed != 12.0 {
		t.Fatalf("signal-family seq 2 = gear %d brake %v speed %v, want 3, 0.4, 12.0", gear, brake, speed)
	}
}

func TestImportAllWritesWheelTyreAndBrakeFamilies(t *testing.T) {
	outputPath := importMinimalFixture(t)

	db, err := sql.Open("duckdb", outputPath+"?access_mode=READ_ONLY")
	if err != nil {
		t.Fatalf("open output DB: %v", err)
	}
	defer db.Close()

	for _, table := range []string{"wheel_dynamics", "tyre_state", "brake_state"} {
		assertTableCount(t, db, table, 4)
	}

	var wheelSpeedFL, suspensionDeflectionRR, rideHeightFR, brakesForceRL float64
	var surfaceTypeRR int64
	if err := db.QueryRow(`
		SELECT wheel_speed_fl, suspension_deflection_rr, ride_height_fr, brakes_force_rl, surface_type_rr
		FROM wheel_dynamics WHERE sample_seq = 2
	`).Scan(&wheelSpeedFL, &suspensionDeflectionRR, &rideHeightFR, &brakesForceRL, &surfaceTypeRR); err != nil {
		t.Fatalf("read wheel_dynamics seq 2: %v", err)
	}
	if wheelSpeedFL != 31 || suspensionDeflectionRR != 0.8 || rideHeightFR != 0.32 || brakesForceRL != 103 || surfaceTypeRR != 7 {
		t.Fatalf("wheel_dynamics seq 2 = speedFL %v suspRR %v rideFR %v forceRL %v surfaceRR %d",
			wheelSpeedFL, suspensionDeflectionRR, rideHeightFR, brakesForceRL, surfaceTypeRR)
	}

	var pressureFL, wearFR, rubberTempRL, carcassRR, rimFL, tempLeftFR, tempCentreRL, tempRightRR float64
	if err := db.QueryRow(`
		SELECT tyres_pressure_fl, tyres_wear_fr, tyres_rubber_temp_rl, carcass_temp_rr, rim_temp_fl,
		       tyres_temp_left_fr, tyres_temp_centre_rl, tyres_temp_right_rr
		FROM tyre_state WHERE sample_seq = 2
	`).Scan(&pressureFL, &wearFR, &rubberTempRL, &carcassRR, &rimFL, &tempLeftFR, &tempCentreRL, &tempRightRR); err != nil {
		t.Fatalf("read tyre_state seq 2: %v", err)
	}
	if pressureFL != 21 || wearFR != 0.22 || rubberTempRL != 63 || carcassRR != 74 || rimFL != 81 || tempLeftFR != 52 || tempCentreRL != 57 || tempRightRR != 61 {
		t.Fatalf("tyre_state seq 2 = pressureFL %v wearFR %v rubberRL %v carcassRR %v rimFL %v tempLeftFR %v tempCentreRL %v tempRightRR %v, want 21, 0.22, 63, 74, 81, 52, 57, 61",
			pressureFL, wearFR, rubberTempRL, carcassRR, rimFL, tempLeftFR, tempCentreRL, tempRightRR)
	}

	var brakeTempFL, brakeAirFR, brakeThicknessRR float64
	if err := db.QueryRow(`
		SELECT brakes_temp_fl, brakes_air_temp_fr, brake_thickness_rr
		FROM brake_state WHERE sample_seq = 2
	`).Scan(&brakeTempFL, &brakeAirFR, &brakeThicknessRR); err != nil {
		t.Fatalf("read brake_state seq 2: %v", err)
	}
	if brakeTempFL != 401 || brakeAirFR != 302 || brakeThicknessRR != 28 {
		t.Fatalf("brake_state seq 2 = tempFL %v airFR %v thicknessRR %v, want 401, 302, 28",
			brakeTempFL, brakeAirFR, brakeThicknessRR)
	}

	var seq2WheelSpeedFL, seq2PressureFR, seq2RubberTempRL, seq2BrakeTempRR float64
	if err := db.QueryRow(`SELECT wheel_speed_fl FROM wheel_dynamics WHERE sample_seq = 2`).Scan(&seq2WheelSpeedFL); err != nil {
		t.Fatalf("read wheel_speed_fl seq 2: %v", err)
	}
	if err := db.QueryRow(`SELECT tyres_pressure_fr, tyres_rubber_temp_rl FROM tyre_state WHERE sample_seq = 2`).Scan(&seq2PressureFR, &seq2RubberTempRL); err != nil {
		t.Fatalf("read tyre_state seq 2 wheel fields: %v", err)
	}
	if err := db.QueryRow(`SELECT brakes_temp_rr FROM brake_state WHERE sample_seq = 2`).Scan(&seq2BrakeTempRR); err != nil {
		t.Fatalf("read brakes_temp_rr seq 2: %v", err)
	}
	if seq2WheelSpeedFL != 31 || seq2PressureFR != 22 || seq2RubberTempRL != 63 || seq2BrakeTempRR != 404 {
		t.Fatalf("wheel-family seq 2 = speedFL %v pressureFR %v rubberRL %v brakeRR %v, want 31, 22, 63, 404",
			seq2WheelSpeedFL, seq2PressureFR, seq2RubberTempRL, seq2BrakeTempRR)
	}
}

func TestImportAllWritesRemainingFamiliesProfilesAndLongtail(t *testing.T) {
	outputPath := importMinimalFixture(t)

	db, err := sql.Open("duckdb", outputPath+"?access_mode=READ_ONLY")
	if err != nil {
		t.Fatalf("open output DB: %v", err)
	}
	defer db.Close()

	for _, table := range []string{
		"aero_platform",
		"energy_state",
		"powertrain_state",
		"environment_state",
		"car_state",
		"damage_state",
		"opponent_context",
	} {
		assertTableCount(t, db, table, 4)
	}

	var frontRH, rearRH, front3rd, rear3rd float64
	if err := db.QueryRow(`
		SELECT aero_front_ride_height, aero_rear_ride_height, aero_front_3rd_deflection, aero_rear_3rd_deflection
		FROM aero_platform WHERE sample_seq = 2
	`).Scan(&frontRH, &rearRH, &front3rd, &rear3rd); err != nil {
		t.Fatalf("read aero_platform seq 2: %v", err)
	}
	if frontRH != 0.14 || rearRH != 0.24 || front3rd != 0.034 || rear3rd != 0.044 {
		t.Fatalf("aero_platform seq 2 = %v %v %v %v, want 0.14 0.24 0.034 0.044", frontRH, rearRH, front3rd, rear3rd)
	}

	var fuelLevel, soc, regen, virtualEnergy float64
	if err := db.QueryRow(`
		SELECT fuel_level, battery_soc, ers_regen_kw, ers_virtual_energy
		FROM energy_state WHERE sample_seq = 2
	`).Scan(&fuelLevel, &soc, &regen, &virtualEnergy); err != nil {
		t.Fatalf("read energy_state seq 2: %v", err)
	}
	if fuelLevel != 47 || soc != 0.62 || regen != 12 || virtualEnergy != 3.2 {
		t.Fatalf("energy_state seq 2 = %v %v %v %v, want 47 0.62 12 3.2", fuelLevel, soc, regen, virtualEnergy)
	}

	var oilTemp, waterTemp float64
	if err := db.QueryRow(`SELECT engine_oil_temp, engine_water_temp FROM powertrain_state WHERE sample_seq = 2`).Scan(&oilTemp, &waterTemp); err != nil {
		t.Fatalf("read powertrain_state seq 2: %v", err)
	}
	if oilTemp != 92 || waterTemp != 82 {
		t.Fatalf("powertrain_state seq 2 = oil %v water %v, want 92 82", oilTemp, waterTemp)
	}

	var airTemp, trackTemp, windSpeed, darkCloud, wetMin, wetAvg float64
	if err := db.QueryRow(`
		SELECT air_temp, track_temp, wind_speed, dark_cloud, path_wetness_min, path_wetness_avg
		FROM environment_state WHERE sample_seq = 2
	`).Scan(&airTemp, &trackTemp, &windSpeed, &darkCloud, &wetMin, &wetAvg); err != nil {
		t.Fatalf("read environment_state seq 2: %v", err)
	}
	if airTemp != 24 || trackTemp != 34 || windSpeed != 6 || darkCloud != 0.4 || wetMin != 0.2 || wetAvg != 0.7 {
		t.Fatalf("environment_state seq 2 = air %v track %v wind %v cloud %v wetMin %v wetAvg %v", airTemp, trackTemp, windSpeed, darkCloud, wetMin, wetAvg)
	}

	var absLevel, tcLevel, tcCut, migration, motorMap, speedLimiter, inPits, headlights int64
	var rearBrakeBias float64
	if err := db.QueryRow(`
		SELECT abs_level, tc_level, tc_cut, migration, motor_map, speed_limiter, in_pits, headlights, rear_brake_bias
		FROM car_state WHERE sample_seq = 2
	`).Scan(&absLevel, &tcLevel, &tcCut, &migration, &motorMap, &speedLimiter, &inPits, &headlights, &rearBrakeBias); err != nil {
		t.Fatalf("read car_state seq 2: %v", err)
	}
	if absLevel != 4 || tcLevel != 6 || tcCut != 2 || migration != 9 || motorMap != 3 || speedLimiter != 1 || inPits != 0 || headlights != 2 || rearBrakeBias != 54 {
		t.Fatalf("car_state seq 2 = abs %d tc %d cut %d migration %d motor %d limiter %d pits %d headlights %d bias %v",
			absLevel, tcLevel, tcCut, migration, motorMap, speedLimiter, inPits, headlights, rearBrakeBias)
	}

	var lastImpact float64
	var wheelDetachedRR int64
	if err := db.QueryRow(`SELECT last_impact_magnitude, wheel_detached_rr FROM damage_state WHERE sample_seq = 2`).Scan(&lastImpact, &wheelDetachedRR); err != nil {
		t.Fatalf("read damage_state seq 2: %v", err)
	}
	if lastImpact != 125 || wheelDetachedRR != 1 {
		t.Fatalf("damage_state seq 2 = impact %v detachedRR %d, want 125 and 1", lastImpact, wheelDetachedRR)
	}

	var aheadGap float64
	if err := db.QueryRow(`SELECT opp_ahead_gap_time FROM opponent_context WHERE sample_seq = 2`).Scan(&aheadGap); err != nil {
		t.Fatalf("read opponent_context seq 2: %v", err)
	}
	if aheadGap != 1.4 {
		t.Fatalf("opponent_context seq 2 opp_ahead_gap_time = %v, want 1.4", aheadGap)
	}

	var storageFamily, storageColumn, mode, source string
	var declaredHz float64
	if err := db.QueryRow(`
		SELECT storage_family, storage_column, declared_hz, mode, source
		FROM channel_profiles WHERE channel_name = 'Wheel Speed'
	`).Scan(&storageFamily, &storageColumn, &declaredHz, &mode, &source); err != nil {
		t.Fatalf("read Wheel Speed channel profile: %v", err)
	}
	if storageFamily != "wheel_dynamics" || storageColumn != "wheel_speed" || declaredHz != 4 || mode != "sampled" || source != "lmu-official-import" {
		t.Fatalf("Wheel Speed profile = family %q column %q hz %v mode %q source %q", storageFamily, storageColumn, declaredHz, mode, source)
	}
	if err := db.QueryRow(`
		SELECT storage_family, storage_column, declared_hz, mode, source
		FROM channel_profiles WHERE channel_name = 'ABSLevel'
	`).Scan(&storageFamily, &storageColumn, &declaredHz, &mode, &source); err != nil {
		t.Fatalf("read ABSLevel channel profile: %v", err)
	}
	if storageFamily != "car_state" || storageColumn != "abs_level" || declaredHz != 1 || mode != "on-change" || source != "lmu-official-import" {
		t.Fatalf("ABSLevel profile = family %q column %q hz %v mode %q source %q", storageFamily, storageColumn, declaredHz, mode, source)
	}

	var gpsSpeed float64
	if err := db.QueryRow(`SELECT value FROM channel_longtail_scalar WHERE channel_name = 'GPS Speed' AND sample_seq = 2`).Scan(&gpsSpeed); err != nil {
		t.Fatalf("read GPS Speed longtail: %v", err)
	}
	if gpsSpeed != 202 {
		t.Fatalf("GPS Speed longtail seq 2 = %v, want 202", gpsSpeed)
	}
	var yellowFlag int64
	if err := db.QueryRow(`SELECT value FROM channel_longtail_int WHERE channel_name = 'Yellow Flag State' AND sample_seq = 2`).Scan(&yellowFlag); err != nil {
		t.Fatalf("read Yellow Flag State longtail: %v", err)
	}
	if yellowFlag != 1 {
		t.Fatalf("Yellow Flag State longtail seq 2 = %d, want 1", yellowFlag)
	}
}

func TestImportAllChannels(t *testing.T) {
	path := findSampleFile(t)

	lmu, err := OpenLMUFile(path)
	if err != nil {
		t.Fatalf("OpenLMUFile error: %v", err)
	}
	defer lmu.Close()

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test.duckdb")

	writer, err := core.NewWriter(outputPath)
	if err != nil {
		t.Fatalf("core.NewWriter error: %v", err)
	}

	stats, err := ImportAll(context.Background(), lmu, writer, WithVerbose(true))
	if err != nil {
		t.Fatalf("ImportAll error: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Writer Close error: %v", err)
	}

	// Verify import stats
	t.Logf("Import stats: processed=%d skipped=%d scalar=%d event=%d wheel=%d event-wheel=%d",
		stats.TablesProcessed, stats.TablesSkipped, stats.ScalarRows, stats.EventRows, stats.WheelRows, stats.EventWheelRows)

	// All known tables should be processed (not skipped)
	if stats.TablesProcessed == 0 {
		t.Error("Expected at least some tables to be processed")
	}

	// We should have scalar rows (Ground Speed, Engine RPM, etc.)
	if stats.ScalarRows == 0 {
		t.Error("Expected scalar rows to be imported")
	}

	// We should have event rows (Lap, Gear, etc.)
	if stats.EventRows == 0 {
		t.Error("Expected event rows to be imported")
	}

	// We should have wheel rows (Wheel Speed, Brakes Temp, etc.)
	if stats.WheelRows == 0 {
		t.Error("Expected wheel rows to be imported")
	}

	// Verify the output database is valid
	verifyFullOutputDB(t, outputPath)
}

func createMinimalLMUExport(t *testing.T) string {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "lmu-source.duckdb")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("create LMU fixture: %v", err)
	}
	defer db.Close()

	execFixtureSQL(t, db, `CREATE TABLE metadata (key VARCHAR, value VARCHAR)`)
	for key, value := range map[string]string{
		"Version":           "1",
		"DriverName":        "Test Driver",
		"SteamID":           "0",
		"RecordingTime":     "2026-05-03T21_23_18Z",
		"SessionTime":       "00:00:01",
		"SessionType":       "Race",
		"TrackName":         "Test Track",
		"TrackLayout":       "Short",
		"WeatherConditions": "Clear",
		"CarName":           "Test Car",
		"CarClass":          "LMP2",
		"CarSetup":          "{}",
	} {
		execFixtureSQL(t, db, `INSERT INTO metadata VALUES (?, ?)`, key, value)
	}

	execFixtureSQL(t, db, `CREATE TABLE channelsList (channelName VARCHAR, frequency INTEGER, unit VARCHAR)`)
	addScalarFixture(t, db, "Ground Speed", 4, "m/s", 10, 11, 12, 13)
	addScalarFixture(t, db, "Engine RPM", 4, "rpm", 5000, 5100, 5200, 5300)
	addScalarFixture(t, db, "Throttle Pos", 4, "%", 0.2, 0.3, 0.4, 0.5)
	addScalarFixture(t, db, "Brake Pos", 2, "%", 0.0, 0.4)
	addScalarFixture(t, db, "Steering Pos", 4, "deg", -0.1, 0.0, 0.1, 0.2)
	addScalarFixture(t, db, "Clutch Pos", 4, "%", 0, 0, 0, 0)
	addScalarFixture(t, db, "G Force Lat", 4, "g", 0.1, 0.2, 0.3, 0.4)
	addScalarFixture(t, db, "G Force Long", 4, "g", 0.0, 0.1, 0.0, -0.1)
	addScalarFixture(t, db, "G Force Vert", 4, "g", 1.0, 1.0, 1.0, 1.0)
	addScalarFixture(t, db, "FFB Output", 4, "", 0.1, 0.2, 0.3, 0.4)
	addScalarFixture(t, db, "Steering Shaft Torque", 4, "Nm", 1, 2, 3, 4)
	addScalarFixture(t, db, "Clutch RPM", 4, "rpm", 4900, 5000, 5100, 5200)
	addScalarFixture(t, db, "Turbo Boost Pressure", 4, "kPa", 100, 101, 102, 103)
	addScalarFixture(t, db, "Engine Max RPM", 100, "rpm", 8000)
	addScalarFixture(t, db, "Lap Dist", 2, "m", 100, 105)
	addScalarFixture(t, db, "Total Dist", 2, "m", 1000, 1005)
	addScalarFixture(t, db, "Path Lateral", 2, "m", 0.1, 0.2)
	addScalarFixture(t, db, "Track Edge", 2, "m", 4.0, 4.1)
	addScalarFixture(t, db, "FrontRideHeight", 4, "m", 0.1, 0.12, 0.14, 0.16)
	addScalarFixture(t, db, "RearRideHeight", 4, "m", 0.2, 0.22, 0.24, 0.26)
	addScalarFixture(t, db, "Front3rdDeflection", 4, "m", 0.03, 0.032, 0.034, 0.036)
	addScalarFixture(t, db, "Rear3rdDeflection", 4, "m", 0.04, 0.042, 0.044, 0.046)
	addScalarFixture(t, db, "Fuel Level", 4, "l", 49, 48, 47, 46)
	addScalarFixture(t, db, "SoC", 4, "", 0.6, 0.61, 0.62, 0.63)
	addScalarFixture(t, db, "Regen Rate", 4, "kW", 10, 11, 12, 13)
	addScalarFixture(t, db, "Virtual Energy", 4, "MJ", 3.0, 3.1, 3.2, 3.3)
	addScalarFixture(t, db, "Engine Oil Temp", 4, "C", 90, 91, 92, 93)
	addScalarFixture(t, db, "Engine Water Temp", 4, "C", 80, 81, 82, 83)
	addScalarFixture(t, db, "Ambient Temperature", 4, "C", 22, 23, 24, 25)
	addScalarFixture(t, db, "Track Temperature", 4, "C", 32, 33, 34, 35)
	addScalarFixture(t, db, "Wind Speed", 4, "m/s", 4, 5, 6, 7)
	addScalarFixture(t, db, "Wind Heading", 4, "deg", 120, 121, 122, 123)
	addScalarFixture(t, db, "GPS Speed", 4, "km/h", 200, 201, 202, 203)
	addScalarFixture(t, db, "Time Behind Next", 4, "s", 1.2, 1.3, 1.4, 1.5)

	addWheelFixture(t, db, "Wheel Speed", 4, "m/s",
		WheelRow{29, 30, 31, 32},
		WheelRow{30, 31, 32, 33},
		WheelRow{31, 32, 33, 34},
		WheelRow{32, 33, 34, 35},
	)
	addWheelFixture(t, db, "Susp Pos", 4, "m",
		WheelRow{0.1, 0.2, 0.3, 0.4},
		WheelRow{0.2, 0.3, 0.4, 0.5},
		WheelRow{0.3, 0.4, 0.7, 0.8},
		WheelRow{0.4, 0.5, 0.8, 0.9},
	)
	addWheelFixture(t, db, "RideHeights", 4, "m",
		WheelRow{0.2, 0.22, 0.24, 0.26},
		WheelRow{0.25, 0.27, 0.29, 0.31},
		WheelRow{0.3, 0.32, 0.34, 0.36},
		WheelRow{0.35, 0.37, 0.39, 0.41},
	)
	addWheelFixture(t, db, "Brakes Force", 4, "N",
		WheelRow{90, 91, 92, 93},
		WheelRow{95, 96, 97, 98},
		WheelRow{101, 102, 103, 104},
		WheelRow{105, 106, 107, 108},
	)
	addWheelFixture(t, db, "TyresPressure", 4, "kPa",
		WheelRow{19, 20, 21, 22},
		WheelRow{20, 21, 22, 23},
		WheelRow{21, 22, 23, 24},
		WheelRow{22, 23, 24, 25},
	)
	addWheelFixture(t, db, "TyresWear", 4, "%",
		WheelRow{90, 89, 88, 87},
		WheelRow{85, 84, 83, 82},
		WheelRow{79, 78, 77, 76},
		WheelRow{70, 69, 68, 67},
	)
	addWheelFixture(t, db, "TyresRubberTemp", 4, "C",
		WheelRow{55, 56, 57, 58},
		WheelRow{58, 59, 60, 61},
		WheelRow{61, 62, 63, 64},
		WheelRow{64, 65, 66, 67},
	)
	addWheelFixture(t, db, "TyresCarcassTemp", 4, "C",
		WheelRow{65, 66, 67, 68},
		WheelRow{68, 69, 70, 71},
		WheelRow{71, 72, 73, 74},
		WheelRow{74, 75, 76, 77},
	)
	addWheelFixture(t, db, "TyresRimTemp", 4, "C",
		WheelRow{75, 76, 77, 78},
		WheelRow{78, 79, 80, 81},
		WheelRow{81, 82, 83, 84},
		WheelRow{84, 85, 86, 87},
	)
	addWheelFixture(t, db, "TyresTempLeft", 4, "C",
		WheelRow{49, 50, 51, 52},
		WheelRow{50, 51, 52, 53},
		WheelRow{51, 52, 53, 54},
		WheelRow{52, 53, 54, 55},
	)
	addWheelFixture(t, db, "TyresTempCentre", 4, "C",
		WheelRow{53, 54, 55, 56},
		WheelRow{54, 55, 56, 57},
		WheelRow{55, 56, 57, 58},
		WheelRow{56, 57, 58, 59},
	)
	addWheelFixture(t, db, "TyresTempRight", 4, "C",
		WheelRow{56, 57, 58, 59},
		WheelRow{57, 58, 59, 60},
		WheelRow{58, 59, 60, 61},
		WheelRow{59, 60, 61, 62},
	)
	addWheelFixture(t, db, "Brakes Temp", 4, "C",
		WheelRow{390, 391, 392, 393},
		WheelRow{395, 396, 397, 398},
		WheelRow{401, 402, 403, 404},
		WheelRow{405, 406, 407, 408},
	)
	addWheelFixture(t, db, "Brakes Air Temp", 4, "C",
		WheelRow{290, 291, 292, 293},
		WheelRow{295, 296, 297, 298},
		WheelRow{301, 302, 303, 304},
		WheelRow{305, 306, 307, 308},
	)
	addWheelFixture(t, db, "Brake Thickness", 4, "mm",
		WheelRow{31, 31, 30, 30},
		WheelRow{30, 30, 29, 29},
		WheelRow{29, 29, 28, 28},
		WheelRow{28, 28, 27, 27},
	)

	execFixtureSQL(t, db, `CREATE TABLE eventsList (eventName VARCHAR, unit VARCHAR)`)
	addEventFixture(t, db, "Gear", "", []eventFixtureRow{{0.0, 2}, {0.5, 3}})
	addEventFixture(t, db, "ABS", "", []eventFixtureRow{{0.0, 0}})
	addEventFixture(t, db, "TC", "", []eventFixtureRow{{0.0, 1}})
	addEventFixture(t, db, "Lap", "", []eventFixtureRow{{0.0, 1}})
	addEventFixture(t, db, "Current Sector", "", []eventFixtureRow{{0.0, 1}, {0.5, 2}})
	addEventFixture(t, db, "Current LapTime", "s", []eventFixtureRow{{0.0, 0.0}, {0.25, 0.25}, {0.5, 0.5}})
	addEventFixture(t, db, "Position", "", []eventFixtureRow{{0.0, 5}})
	addEventFixture(t, db, "Class Position", "", []eventFixtureRow{{0.0, 2}})
	addEventFixture(t, db, "Best LapTime", "s", []eventFixtureRow{{0.0, 80.1}})
	addEventFixture(t, db, "Best Sector1", "s", []eventFixtureRow{{0.0, 25.1}})
	addEventFixture(t, db, "Best Sector2", "s", []eventFixtureRow{{0.0, 55.2}})
	addEventFixture(t, db, "Lap Time", "s", []eventFixtureRow{{0.0, 82.0}})
	addEventFixture(t, db, "Last Sector1", "s", []eventFixtureRow{{0.0, 26.0}})
	addEventFixture(t, db, "Last Sector2", "s", []eventFixtureRow{{0.0, 56.0}})
	addEventFixture(t, db, "Delta Best", "s", []eventFixtureRow{{0.0, 0.1}, {0.5, -0.2}})
	addEventFixture(t, db, "Current Sector1", "s", []eventFixtureRow{{0.0, 24.9}})
	addEventFixture(t, db, "Current Sector2", "s", []eventFixtureRow{{0.5, 54.8}})
	addEventFixture(t, db, "ABSLevel", "", []eventFixtureRow{{0.0, 3}, {0.5, 4}})
	addEventFixture(t, db, "TCLevel", "", []eventFixtureRow{{0.0, 5}, {0.5, 6}})
	addEventFixture(t, db, "TCCut", "", []eventFixtureRow{{0.0, 1}, {0.5, 2}})
	addEventFixture(t, db, "Brake Migration", "", []eventFixtureRow{{0.0, 8}, {0.5, 9}})
	addEventFixture(t, db, "FuelMixtureMap", "", []eventFixtureRow{{0.0, 2}, {0.5, 3}})
	addEventFixture(t, db, "Speed Limiter", "", []eventFixtureRow{{0.0, 0}, {0.5, 1}})
	addEventFixture(t, db, "In Pits", "", []eventFixtureRow{{0.0, 0}})
	addEventFixture(t, db, "Headlights State", "", []eventFixtureRow{{0.0, 1}, {0.5, 2}})
	addEventFixture(t, db, "Brake Bias Rear", "%", []eventFixtureRow{{0.0, 53}, {0.5, 54}})
	addEventFixture(t, db, "CloudDarkness", "", []eventFixtureRow{{0.0, 0.2}, {0.5, 0.4}})
	addEventFixture(t, db, "Minimum Path Wetness", "", []eventFixtureRow{{0.0, 0.1}, {0.5, 0.2}})
	addEventFixture(t, db, "OffpathWetness", "", []eventFixtureRow{{0.0, 0.5}, {0.5, 0.7}})
	addEventFixture(t, db, "LastImpactMagnitude", "N", []eventFixtureRow{{0.0, 0}, {0.5, 125}})
	addEventFixture(t, db, "Yellow Flag State", "", []eventFixtureRow{{0.0, 0}, {0.5, 1}})
	addEventWheelFixture(t, db, "SurfaceTypes", "",
		[]EventWheelRow{{Ts: 0.0, V1: 1, V2: 1, V3: 1, V4: 1}, {Ts: 0.5, V1: 4, V2: 5, V3: 6, V4: 7}},
	)
	addEventWheelFixture(t, db, "WheelsDetached", "",
		[]EventWheelRow{{Ts: 0.0, V1: 0, V2: 0, V3: 0, V4: 0}, {Ts: 0.5, V1: 0, V2: 0, V3: 0, V4: 1}},
	)

	return dbPath
}

func importMinimalFixture(t *testing.T) string {
	t.Helper()

	inputPath := createMinimalLMUExport(t)
	lmu, err := OpenLMUFile(inputPath)
	if err != nil {
		t.Fatalf("OpenLMUFile error: %v", err)
	}
	defer lmu.Close()

	outputPath := filepath.Join(t.TempDir(), "signal-family.duckdb")
	writer, err := core.NewWriter(outputPath)
	if err != nil {
		t.Fatalf("core.NewWriter error: %v", err)
	}
	if _, err := ImportAll(context.Background(), lmu, writer); err != nil {
		t.Fatalf("ImportAll error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Writer Close error: %v", err)
	}

	return outputPath
}

func addScalarFixture(t *testing.T, db *sql.DB, name string, frequency int, unit string, values ...float64) {
	t.Helper()
	execFixtureSQL(t, db, `INSERT INTO channelsList VALUES (?, ?, ?)`, name, frequency, unit)
	execFixtureSQL(t, db, fmt.Sprintf(`CREATE TABLE "%s" (value DOUBLE)`, name))
	for _, value := range values {
		execFixtureSQL(t, db, fmt.Sprintf(`INSERT INTO "%s" VALUES (?)`, name), value)
	}
}

func addWheelFixture(t *testing.T, db *sql.DB, name string, frequency int, unit string, rows ...WheelRow) {
	t.Helper()
	execFixtureSQL(t, db, `INSERT INTO channelsList VALUES (?, ?, ?)`, name, frequency, unit)
	execFixtureSQL(t, db, fmt.Sprintf(`CREATE TABLE "%s" (value1 DOUBLE, value2 DOUBLE, value3 DOUBLE, value4 DOUBLE)`, name))
	for _, row := range rows {
		execFixtureSQL(t, db, fmt.Sprintf(`INSERT INTO "%s" VALUES (?, ?, ?, ?)`, name), row.V1, row.V2, row.V3, row.V4)
	}
}

type eventFixtureRow struct {
	ts    float64
	value float64
}

func addEventFixture(t *testing.T, db *sql.DB, name string, unit string, rows []eventFixtureRow) {
	t.Helper()
	execFixtureSQL(t, db, `INSERT INTO eventsList VALUES (?, ?)`, name, unit)
	execFixtureSQL(t, db, fmt.Sprintf(`CREATE TABLE "%s" (ts DOUBLE, value DOUBLE)`, name))
	for _, row := range rows {
		execFixtureSQL(t, db, fmt.Sprintf(`INSERT INTO "%s" VALUES (?, ?)`, name), row.ts, row.value)
	}
}

func addEventWheelFixture(t *testing.T, db *sql.DB, name string, unit string, rows []EventWheelRow) {
	t.Helper()
	execFixtureSQL(t, db, `INSERT INTO eventsList VALUES (?, ?)`, name, unit)
	execFixtureSQL(t, db, fmt.Sprintf(`CREATE TABLE "%s" (ts DOUBLE, value1 DOUBLE, value2 DOUBLE, value3 DOUBLE, value4 DOUBLE)`, name))
	for _, row := range rows {
		execFixtureSQL(t, db, fmt.Sprintf(`INSERT INTO "%s" VALUES (?, ?, ?, ?, ?)`, name), row.Ts, row.V1, row.V2, row.V3, row.V4)
	}
}

func execFixtureSQL(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("fixture SQL failed: %s: %v", query, err)
	}
}

func testTableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'main' AND table_name = ?`, table).Scan(&count); err != nil {
		t.Fatalf("check table %q: %v", table, err)
	}
	return count > 0
}

func assertTableCount(t *testing.T, db *sql.DB, table string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, table)).Scan(&got); err != nil {
		t.Fatalf("count %q: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s row count = %d, want %d", table, got, want)
	}
}

func TestImportAllDryRun(t *testing.T) {
	path := findSampleFile(t)

	lmu, err := OpenLMUFile(path)
	if err != nil {
		t.Fatalf("OpenLMUFile error: %v", err)
	}
	defer lmu.Close()

	stats, err := ImportAll(context.Background(), lmu, nil, WithDryRun(true), WithVerbose(true))
	if err != nil {
		t.Fatalf("ImportAll dry-run error: %v", err)
	}

	// Dry run should still count rows (reads data, just doesn't write)
	if stats.TablesProcessed == 0 {
		t.Error("Expected dry-run to still process tables")
	}
	if stats.ScalarRows == 0 {
		t.Error("Expected dry-run to still count scalar rows")
	}
}

func verifyFullOutputDB(t *testing.T, dbPath string) {
	t.Helper()

	dsn := dbPath + "?access_mode=READ_ONLY"
	db, err := sql.Open("duckdb", dsn)
	if err != nil {
		t.Fatalf("open output DB: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Verify lapace_version exists with canonical schema
	var schemaVersion string
	var schemaType string
	var dataVersion int
	err = db.QueryRowContext(ctx, `SELECT schema_version, data_version, schema_type FROM lapace_version LIMIT 1`).Scan(&schemaVersion, &dataVersion, &schemaType)
	if err != nil {
		t.Fatalf("read lapace_version: %v", err)
	}
	if schemaVersion == "" {
		t.Error("lapace_version.schema_version should not be empty")
	}
	if dataVersion <= 0 {
		t.Errorf("lapace_version.data_version = %d, want positive version", dataVersion)
	}
	if schemaType != schema.SignalFamilySchemaType {
		t.Errorf("lapace_version.schema_type = %q, want %q", schemaType, schema.SignalFamilySchemaType)
	}

	// Verify session_metadata exists with canonical column types
	var trackName, sessionType string
	err = db.QueryRowContext(ctx, `SELECT track_name, session_type FROM session_metadata LIMIT 1`).Scan(&trackName, &sessionType)
	if err != nil {
		t.Fatalf("read session_metadata: %v", err)
	}
	t.Logf("session_metadata: track=%q type=%q", trackName, sessionType)

	// Verify canonical column types: recorded_at TIMESTAMP, frame_count BIGINT, sample_rate_hz DOUBLE
	colTypes := map[string]string{
		"recorded_at":    "TIMESTAMP",
		"frame_count":    "BIGINT",
		"sample_rate_hz": "DOUBLE",
	}
	for col, wantType := range colTypes {
		var colType string
		err = db.QueryRowContext(ctx,
			`SELECT data_type FROM information_schema.columns WHERE table_name='session_metadata' AND column_name=$1`,
			col).Scan(&colType)
		if err != nil {
			t.Fatalf("check session_metadata column %q type: %v", col, err)
		}
		if colType != wantType {
			t.Errorf("session_metadata.%s type = %q, want %q", col, colType, wantType)
		}
	}

	// Verify recorded_at is a valid timestamp (not epoch double)
	var recordedAt interface{}
	err = db.QueryRowContext(ctx, `SELECT recorded_at FROM session_metadata LIMIT 1`).Scan(&recordedAt)
	if err != nil {
		t.Fatalf("read recorded_at: %v", err)
	}
	t.Logf("session_metadata.recorded_at: %v (type: %T)", recordedAt, recordedAt)

	for _, table := range schema.SignalFamilyTableNames {
		if !testTableExists(t, db, table) {
			t.Fatalf("signal-family table %q was not created", table)
		}
	}
	if testTableExists(t, db, "Ground Speed") {
		t.Fatal("legacy narrow table Ground Speed should not be written")
	}

	for _, table := range []string{
		"source_samples",
		"driver_controls",
		"vehicle_dynamics",
		"powertrain",
		"progress_position",
		"race_standing",
		"lap_reference_timing",
		"current_lap_timing",
	} {
		var count int
		err = db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, table)).Scan(&count)
		if err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count == 0 {
			t.Errorf("%s table is empty", table)
		}
	}

	// Verify timestamp reconstruction: scalar ts should be session-relative (near 0.0)
	var firstTS float64
	err = db.QueryRowContext(ctx, `SELECT session_ts_raw FROM source_samples ORDER BY sample_seq LIMIT 1`).Scan(&firstTS)
	if err != nil {
		t.Fatalf("read first ts: %v", err)
	}
	if firstTS < 0.0 || firstTS > 1.0 {
		t.Errorf("first ts = %f, expected session-relative (near 0.0)", firstTS)
	}

	t.Logf("signal-family output verified: firstTS=%f ver=%q", firstTS, schemaVersion)
}
