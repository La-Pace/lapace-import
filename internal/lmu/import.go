package lmu

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"github.com/La-Pace/lapace-import/internal/core"
	schema "github.com/La-Pace/lapace-import/internal/schema"
)

const SourceLabel = "lmu-official-import"

// ImportStats holds counters for an import run.
type ImportStats struct {
	TablesProcessed int
	TablesSkipped   int
	ScalarRows      int
	EventRows       int
	WheelRows       int
	EventWheelRows  int
	UnknownTables   []string
}

// ImportAll reads all telemetry channels from an LMU DuckDB export file and
// writes a signal-family Lapace DuckDB. It builds a dense source sample spine
// from the highest-frequency continuous LMU channel and projects LMU channels
// directly into SignalFamilyRows without routing through LapaceFrame.
func ImportAll(ctx context.Context, lmu *LMUFile, writer *core.Writer, opts ...ImportOption) (*ImportStats, error) {
	cfg := defaultImportConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	meta, data, stats, err := readImportedTelemetry(ctx, lmu, cfg)
	if err != nil {
		return nil, err
	}

	if cfg.dryRun {
		return stats, nil
	}
	if writer == nil {
		return nil, fmt.Errorf("writer is required unless dry-run is enabled")
	}

	rows, err := buildSignalFamilyRows(meta, data)
	if err != nil {
		return nil, err
	}

	if cfg.writeMetadata {
		phase := cfg.phaseOverride
		if phase == "" {
			derivedPhase, _ := DerivePhase(meta.SessionType)
			phase = derivedPhase.Slug()
		}
		recordedAt, _ := ParseRecordingTimeAsTime(meta.RecordingTime)
		sessionID := fmt.Sprintf("import-%s", meta.RecordingTime)
		if err := writer.WriteSessionMetadata(sessionID, phase, meta.TrackName, meta.CarName, meta.CarClass, meta.DriverName, meta.TrackLayout, recordedAt); err != nil {
			return nil, fmt.Errorf("write session metadata: %w", err)
		}
		if cfg.verbose {
			log.Printf("  session_metadata: track=%q type=%q driver=%q", meta.TrackName, phase, meta.DriverName)
		}
	}

	if err := writer.WriteSignalFamilyRows(ctx, rows); err != nil {
		return nil, fmt.Errorf("write signal-family rows: %w", err)
	}
	if cfg.verbose {
		log.Printf("  signal-family rows: %d source samples", len(rows))
	}

	return stats, nil
}

func readImportedTelemetry(ctx context.Context, lmu *LMUFile, cfg importConfig) (*LMUMetadata, *importedTelemetry, *ImportStats, error) {
	meta, err := lmu.Metadata()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read LMU metadata: %w", err)
	}

	channels, err := lmu.ChannelsList()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read channelsList: %w", err)
	}

	events, err := lmu.EventsList()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read eventsList: %w", err)
	}

	tables, err := lmu.ChannelTables()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("list channel tables: %w", err)
	}

	freqMap := make(map[string]int)
	for _, ch := range channels {
		freqMap[ch.Name] = ch.Frequency
	}
	for _, ev := range events {
		if freqMap[ev.Name] == 0 {
			freqMap[ev.Name] = 1
		}
	}

	data := newImportedTelemetry()
	stats := &ImportStats{}

	for _, name := range tables {
		switch CategorizeTable(name) {
		case CategoryScalar:
			if err := readScalarSeries(ctx, lmu, name, freqMap, data, stats, cfg); err != nil {
				return nil, nil, nil, err
			}
		case CategoryEvent:
			if err := readEventSeries(ctx, lmu, name, data, stats, cfg); err != nil {
				return nil, nil, nil, err
			}
		case CategoryWheel:
			if err := readWheelSeries(ctx, lmu, name, freqMap, data, stats, cfg); err != nil {
				return nil, nil, nil, err
			}
		case CategoryEventWheel:
			if err := readEventWheelSeries(ctx, lmu, name, data, stats, cfg); err != nil {
				return nil, nil, nil, err
			}
		default:
			stats.TablesSkipped++
			stats.UnknownTables = append(stats.UnknownTables, name)
			if cfg.verbose {
				log.Printf("  SKIP unknown table %q", name)
			}
		}
	}
	return meta, data, stats, nil
}

type scalarSeries struct {
	frequency int
	values    []float64
}

type wheelSeries struct {
	frequency int
	values    []WheelRow
}

type importedTelemetry struct {
	scalars     map[string]scalarSeries
	events      map[string][]EventRow
	wheels      map[string]wheelSeries
	eventWheels map[string][]EventWheelRow
}

func newImportedTelemetry() *importedTelemetry {
	return &importedTelemetry{
		scalars:     make(map[string]scalarSeries),
		events:      make(map[string][]EventRow),
		wheels:      make(map[string]wheelSeries),
		eventWheels: make(map[string][]EventWheelRow),
	}
}

func readScalarSeries(ctx context.Context, lmu *LMUFile, name string, freqMap map[string]int, data *importedTelemetry, stats *ImportStats, cfg importConfig) error {
	freq := freqMap[name]
	if freq == 0 {
		if cfg.verbose {
			log.Printf("  SKIP scalar %q: no frequency in channelsList", name)
		}
		stats.TablesSkipped++
		return nil
	}

	rows, err := lmu.ReadScalarTable(ctx, name)
	if err != nil {
		return fmt.Errorf("read scalar table %q: %w", name, err)
	}
	values := make([]float64, len(rows))
	for i, row := range rows {
		values[i] = row.Value
	}
	data.scalars[name] = scalarSeries{frequency: freq, values: values}

	stats.TablesProcessed++
	stats.ScalarRows += len(rows)
	if cfg.verbose {
		log.Printf("  SCALAR %q: %d rows at %dHz", name, len(rows), freq)
	}
	return nil
}

func readEventSeries(ctx context.Context, lmu *LMUFile, name string, data *importedTelemetry, stats *ImportStats, cfg importConfig) error {
	rows, err := lmu.ReadEventTable(ctx, name)
	if err != nil {
		return fmt.Errorf("read event table %q: %w", name, err)
	}
	data.events[name] = rows

	stats.TablesProcessed++
	stats.EventRows += len(rows)
	if cfg.verbose {
		log.Printf("  EVENT %q: %d rows", name, len(rows))
	}
	return nil
}

func readWheelSeries(ctx context.Context, lmu *LMUFile, name string, freqMap map[string]int, data *importedTelemetry, stats *ImportStats, cfg importConfig) error {
	freq := freqMap[name]
	if freq == 0 {
		if cfg.verbose {
			log.Printf("  SKIP wheel %q: no frequency in channelsList", name)
		}
		stats.TablesSkipped++
		return nil
	}

	rows, err := lmu.ReadWheelTable(ctx, name)
	if err != nil {
		return fmt.Errorf("read wheel table %q: %w", name, err)
	}
	if MapTableName(name) == "Tyres Wear" {
		for i := range rows {
			rows[i] = ConvertWearFraction(rows[i])
		}
	}
	data.wheels[name] = wheelSeries{frequency: freq, values: rows}

	stats.TablesProcessed++
	stats.WheelRows += len(rows)
	if cfg.verbose {
		log.Printf("  WHEEL %q: %d rows at %dHz", name, len(rows), freq)
	}
	return nil
}

func readEventWheelSeries(ctx context.Context, lmu *LMUFile, name string, data *importedTelemetry, stats *ImportStats, cfg importConfig) error {
	rows, err := lmu.ReadEventWheelTable(ctx, name)
	if err != nil {
		return fmt.Errorf("read event-wheel table %q: %w", name, err)
	}
	data.eventWheels[name] = rows

	stats.TablesProcessed++
	stats.EventWheelRows += len(rows)
	if cfg.verbose {
		log.Printf("  EVENT-WHEEL %q: %d rows", name, len(rows))
	}
	return nil
}

func buildSignalFamilyRows(meta *LMUMetadata, data *importedTelemetry) ([]schema.SignalFamilyRows, error) {
	sampleCount, sampleRateHz, err := data.sampleSpine()
	if err != nil {
		return nil, err
	}

	recordedAt, err := ParseRecordingTimeAsTime(meta.RecordingTime)
	if err != nil {
		recordedAt = time.Unix(0, 0).UTC()
	}

	rows := make([]schema.SignalFamilyRows, sampleCount)
	deltaTime := 1.0 / float64(sampleRateHz)

	for i := 0; i < sampleCount; i++ {
		sampleSeq := int64(i)
		sampleTime := float64(i) / float64(sampleRateHz)
		capturedAt := recordedAt.Add(time.Duration(sampleTime * float64(time.Second)))
		currentLapTime, ok := data.eventAtAny([]string{"Current LapTime", "Current Lap Time"}, sampleTime)
		if !ok {
			if value, found := data.eventAtAny([]string{"Lap Time"}, sampleTime); found {
				currentLapTime = value
			} else {
				currentLapTime = sampleTime
			}
		}

		gLat := data.scalarAt("G Force Lat", sampleTime)
		gLong := data.scalarAt("G Force Long", sampleTime)
		gVert := data.scalarAt("G Force Vert", sampleTime)
		wheelSpeed := data.wheelAt("Wheel Speed", sampleTime)
		suspensionDeflection := data.wheelAt("Susp Pos", sampleTime)
		rideHeight := data.wheelAt("RideHeights", sampleTime)
		brakesForce := data.wheelAt("Brakes Force", sampleTime)
		surfaceType := data.intEventWheelAt("SurfaceTypes", sampleTime)
		tyresPressure := data.wheelAt("TyresPressure", sampleTime)
		tyresWear := data.wheelAtAny([]string{"TyresWear", "Tyres Wear"}, sampleTime)
		tyresRubberTemp := data.wheelAt("TyresRubberTemp", sampleTime)
		tyresTempLeft := data.wheelAt("TyresTempLeft", sampleTime)
		tyresTempCentre := data.wheelAt("TyresTempCentre", sampleTime)
		tyresTempRight := data.wheelAt("TyresTempRight", sampleTime)
		carcassTemp := data.wheelAt("TyresCarcassTemp", sampleTime)
		rimTemp := data.wheelAt("TyresRimTemp", sampleTime)
		brakesTemp := data.wheelAt("Brakes Temp", sampleTime)
		brakesAirTemp := data.wheelAt("Brakes Air Temp", sampleTime)
		brakeThickness := data.wheelAt("Brake Thickness", sampleTime)
		wheelsDetached := data.intEventWheelAt("WheelsDetached", sampleTime)

		rows[i] = schema.SignalFamilyRows{
			SourceSamples: schema.SourceSamplesRow{
				SampleSeq:      sampleSeq,
				CaptureTSNS:    capturedAt.UnixNano(),
				CapturedAt:     capturedAt,
				SessionTSRaw:   sampleTime,
				CurrentLapTime: currentLapTime,
				GPSTimeRaw:     data.scalarAt("GPS Time", sampleTime),
				DeltaTime:      deltaTime,
				BatchSeq:       0,
				Source:         SourceLabel,
			},
			DriverControls: schema.DriverControlsRow{
				SampleSeq:             sampleSeq,
				ThrottlePos:           data.scalarAt("Throttle Pos", sampleTime),
				BrakePos:              data.scalarAt("Brake Pos", sampleTime),
				SteeringPos:           data.scalarAt("Steering Pos", sampleTime),
				ClutchPos:             data.scalarAt("Clutch Pos", sampleTime),
				ThrottlePosUnfiltered: data.scalarAt("Throttle Pos Unfiltered", sampleTime),
				BrakePosUnfiltered:    data.scalarAt("Brake Pos Unfiltered", sampleTime),
				SteeringPosUnfiltered: data.scalarAt("Steering Pos Unfiltered", sampleTime),
				ClutchPosUnfiltered:   data.scalarAt("Clutch Pos Unfiltered", sampleTime),
				Gear:                  data.intEventAt("Gear", sampleTime),
				ABSActive:             data.intEventAt("ABS", sampleTime),
				TCActive:              data.intEventAt("TC", sampleTime),
			},
			VehicleDynamics: schema.VehicleDynamicsRow{
				SampleSeq:           sampleSeq,
				GroundSpeed:         data.scalarAt("Ground Speed", sampleTime),
				GForceLat:           gLat,
				GForceLong:          gLong,
				GForceVert:          gVert,
				GForceMagnitude:     math.Sqrt(gLat*gLat + gLong*gLong + gVert*gVert),
				FFBOutput:           data.scalarAt("FFB Output", sampleTime),
				SteeringShaftTorque: data.scalarAt("Steering Shaft Torque", sampleTime),
			},
			Powertrain: schema.PowertrainRow{
				SampleSeq:          sampleSeq,
				EngineRPM:          data.scalarAt("Engine RPM", sampleTime),
				ClutchRPM:          data.scalarAt("Clutch RPM", sampleTime),
				TurboBoostPressure: data.scalarAt("Turbo Boost Pressure", sampleTime),
				RPMMax:             data.scalarAt("Engine Max RPM", sampleTime),
			},
			ProgressPosition: schema.ProgressPositionRow{
				SampleSeq:     sampleSeq,
				Lap:           data.intEventAt("Lap", sampleTime),
				CurrentSector: data.intEventAt("Current Sector", sampleTime),
				LapDist:       data.scalarAt("Lap Dist", sampleTime),
				TotalDist:     data.scalarAt("Total Dist", sampleTime),
				LapCompletion: data.scalarAt("Lap Completion", sampleTime),
				PositionX:     data.scalarAt("Position X", sampleTime),
				PositionY:     0, // no world-Y source in LMU exports
				PositionZ:     data.scalarAt("Position Z", sampleTime),
				PathLateral:   data.scalarAt("Path Lateral", sampleTime),
				TrackEdge:     data.scalarAt("Track Edge", sampleTime),
			},
			RaceStanding: schema.RaceStandingRow{
				SampleSeq:     sampleSeq,
				Position:      data.intEventAt("Position", sampleTime),
				ClassPosition: data.intEventAt("Class Position", sampleTime),
			},
			LapReferenceTiming: schema.LapReferenceTimingRow{
				SampleSeq:   sampleSeq,
				BestLapTime: data.eventAt("Best LapTime", sampleTime),
				BestSector1: data.eventAt("Best Sector1", sampleTime),
				BestSector2: data.eventAt("Best Sector2", sampleTime),
				LastLapTime: data.eventAtAnyValue([]string{"Last LapTime", "Lap Time"}, sampleTime),
				LastSector1: data.eventAt("Last Sector1", sampleTime),
				LastSector2: data.eventAt("Last Sector2", sampleTime),
			},
			CurrentLapTiming: schema.CurrentLapTimingRow{
				SampleSeq:      sampleSeq,
				DeltaToBestLap: data.eventAtAnyValue([]string{"Delta Best", "Delta To Best", "Delta to Best"}, sampleTime),
				CurrentSector1: data.eventAt("Current Sector1", sampleTime),
				CurrentSector2: data.eventAt("Current Sector2", sampleTime),
			},
			WheelDynamics: schema.WheelDynamicsRow{
				SampleSeq:            sampleSeq,
				WheelSpeed:           wheelSpeed,
				SuspensionDeflection: suspensionDeflection,
				RideHeight:           rideHeight,
				BrakesForce:          brakesForce,
				SurfaceType:          surfaceType,
			},
			TyreState: schema.TyreStateRow{
				SampleSeq:   sampleSeq,
				Pressure:    tyresPressure,
				Wear:        tyresWear,
				RubberTemp:  tyresRubberTemp,
				TempLeft:    tyresTempLeft,
				TempCentre:  tyresTempCentre,
				TempRight:   tyresTempRight,
				CarcassTemp: carcassTemp,
				RimTemp:     rimTemp,
			},
			BrakeState: schema.BrakeStateRow{
				SampleSeq:      sampleSeq,
				BrakesTemp:     brakesTemp,
				BrakesAirTemp:  brakesAirTemp,
				BrakeThickness: brakeThickness,
			},
			AeroPlatform: schema.AeroPlatformRow{
				SampleSeq:              sampleSeq,
				AeroFrontRideHeight:    data.scalarAt("FrontRideHeight", sampleTime),
				AeroRearRideHeight:     data.scalarAt("RearRideHeight", sampleTime),
				FrontWingHeight:        data.scalarAt("Front Wing Height", sampleTime),
				AeroFront3rdDeflection: data.scalarAt("Front3rdDeflection", sampleTime),
				AeroRear3rdDeflection:  data.scalarAt("Rear3rdDeflection", sampleTime),
				Drag:                   data.scalarAt("Drag", sampleTime),
				FrontDownforce:         data.scalarAt("Front Downforce", sampleTime),
				RearDownforce:          data.scalarAt("Rear Downforce", sampleTime),
				TotalDownforce:         data.scalarAt("Total Downforce", sampleTime),
			},
			EnergyState: schema.EnergyStateRow{
				SampleSeq:        sampleSeq,
				FuelLevel:        data.scalarAt("Fuel Level", sampleTime),
				BatterySOC:       data.scalarAt("SoC", sampleTime),
				ERSRegenKW:       data.scalarAt("Regen Rate", sampleTime),
				ERSVirtualEnergy: data.scalarAt("Virtual Energy", sampleTime),
			},
			PowertrainState: schema.PowertrainStateRow{
				SampleSeq:       sampleSeq,
				EngineOilTemp:   data.scalarAt("Engine Oil Temp", sampleTime),
				EngineWaterTemp: data.scalarAt("Engine Water Temp", sampleTime),
				MotorTemp:       data.scalarAt("Motor Temp", sampleTime),
				ERSWaterTemp:    data.scalarAt("ERS Water Temp", sampleTime),
			},
			OpponentContext: schema.OpponentContextRow{
				SampleSeq:       sampleSeq,
				OppAheadGapTime: data.scalarAt("Time Behind Next", sampleTime),
			},
			EnvironmentState: schema.EnvironmentStateRow{
				SampleSeq:      sampleSeq,
				RainIntensity:  data.scalarAt("Rain Intensity", sampleTime),
				AirTemp:        data.scalarAt("Ambient Temperature", sampleTime),
				TrackTemp:      data.scalarAt("Track Temperature", sampleTime),
				GripPct:        data.scalarAt("Grip Pct", sampleTime),
				DarkCloud:      data.eventAt("CloudDarkness", sampleTime),
				CloudCoverage:  data.scalarAt("Cloud Coverage", sampleTime),
				WindSpeed:      data.scalarAt("Wind Speed", sampleTime),
				WindHeading:    data.scalarAt("Wind Heading", sampleTime),
				PathWetnessMin: data.eventAt("Minimum Path Wetness", sampleTime),
				PathWetnessMax: data.eventAt("OffpathWetness", sampleTime),
				PathWetnessAvg: data.eventAt("OffpathWetness", sampleTime),
			},
			CarState: schema.CarStateRow{
				SampleSeq:     sampleSeq,
				ABSLevel:      data.intEventAt("ABSLevel", sampleTime),
				TCLevel:       data.intEventAt("TCLevel", sampleTime),
				TCSlip:        data.eventAt("TCSlipAngle", sampleTime),
				TCCut:         data.intEventAt("TCCut", sampleTime),
				MotorMap:      data.intEventAt("FuelMixtureMap", sampleTime),
				Migration:     data.intEventAt("Brake Migration", sampleTime),
				DRSActive:     data.intEventAt("RearFlapActivated", sampleTime),
				DRSAvailable:  data.intEventAt("RearFlapLegalStatus", sampleTime),
				InPits:        data.intEventAt("In Pits", sampleTime),
				Overheating:   data.intEventAt("OverheatingState", sampleTime),
				AntiStall:     data.intEventAt("AntiStall Activated", sampleTime),
				FrontFlap:     data.eventAt("FrontFlapActivated", sampleTime),
				RearFlap:      data.eventAt("RearFlapActivated", sampleTime),
				RearFlapLegal: data.intEventAt("RearFlapLegalStatus", sampleTime),
				SpeedLimiter:  data.intEventAt("Speed Limiter", sampleTime),
				Headlights:    data.intEventAt("Headlights State", sampleTime),
				RearBrakeBias: data.eventAt("Brake Bias Rear", sampleTime),
			},
			DamageState: schema.DamageStateRow{
				SampleSeq:           sampleSeq,
				LastImpactMagnitude: data.eventAt("LastImpactMagnitude", sampleTime),
				WheelDetached:       wheelsDetached,
			},
		}
	}

	if len(rows) > 0 {
		rows[0].ChannelProfiles = data.channelProfiles(sampleRateHz)
		rows[0].ChannelLongtailScalar, rows[0].ChannelLongtailWheel4, rows[0].ChannelLongtailInt = data.longtailRows(sampleRateHz)
	}

	return rows, nil
}

func (data *importedTelemetry) sampleSpine() (sampleCount int, sampleRateHz int, err error) {
	consider := func(frequency int, count int, allowSingleSample bool) {
		if count == 0 || frequency <= 0 {
			return
		}
		if !allowSingleSample && count == 1 {
			return
		}
		if frequency > sampleRateHz || (frequency == sampleRateHz && count > sampleCount) {
			sampleRateHz = frequency
			sampleCount = count
		}
	}

	for _, series := range data.scalars {
		consider(series.frequency, len(series.values), false)
	}
	for _, series := range data.wheels {
		consider(series.frequency, len(series.values), false)
	}
	if sampleCount != 0 && sampleRateHz != 0 {
		return sampleCount, sampleRateHz, nil
	}

	for _, series := range data.scalars {
		consider(series.frequency, len(series.values), true)
	}
	for _, series := range data.wheels {
		consider(series.frequency, len(series.values), true)
	}
	if sampleCount == 0 || sampleRateHz == 0 {
		return 0, 0, fmt.Errorf("cannot build source sample spine: no continuous LMU channels with frequency")
	}
	return sampleCount, sampleRateHz, nil
}

func (data *importedTelemetry) scalarAt(name string, sampleTime float64) float64 {
	series, ok := data.scalars[name]
	if !ok || len(series.values) == 0 || series.frequency <= 0 {
		return 0
	}
	idx := int(math.Floor(sampleTime*float64(series.frequency) + 1e-9))
	if idx < 0 {
		return 0
	}
	if idx >= len(series.values) {
		idx = len(series.values) - 1
	}
	return series.values[idx]
}

func (data *importedTelemetry) wheelAt(name string, sampleTime float64) [4]float64 {
	series, ok := data.wheels[name]
	if !ok || len(series.values) == 0 || series.frequency <= 0 {
		return [4]float64{}
	}
	idx := int(math.Floor(sampleTime*float64(series.frequency) + 1e-9))
	if idx < 0 {
		return [4]float64{}
	}
	if idx >= len(series.values) {
		idx = len(series.values) - 1
	}
	row := series.values[idx]
	return [4]float64{row.V1, row.V2, row.V3, row.V4}
}

func (data *importedTelemetry) wheelAtAny(names []string, sampleTime float64) [4]float64 {
	for _, name := range names {
		if _, ok := data.wheels[name]; ok {
			return data.wheelAt(name, sampleTime)
		}
	}
	return [4]float64{}
}

func (data *importedTelemetry) eventAt(name string, sampleTime float64) float64 {
	value, _ := data.eventAtOK(name, sampleTime)
	return value
}

func (data *importedTelemetry) intEventAt(name string, sampleTime float64) int64 {
	value, ok := data.eventAtOK(name, sampleTime)
	if !ok {
		return 0
	}
	return int64(math.Round(value))
}

func (data *importedTelemetry) eventAtAny(names []string, sampleTime float64) (float64, bool) {
	for _, name := range names {
		if value, ok := data.eventAtOK(name, sampleTime); ok {
			return value, true
		}
	}
	return 0, false
}

func (data *importedTelemetry) eventAtAnyValue(names []string, sampleTime float64) float64 {
	value, _ := data.eventAtAny(names, sampleTime)
	return value
}

func (data *importedTelemetry) eventAtOK(name string, sampleTime float64) (float64, bool) {
	rows, ok := data.events[name]
	if !ok || len(rows) == 0 {
		return 0, false
	}
	idx := sort.Search(len(rows), func(i int) bool {
		return rows[i].Ts > sampleTime+1e-9
	}) - 1
	if idx < 0 {
		return 0, false
	}
	return rows[idx].FloatValue, true
}

func (data *importedTelemetry) intEventWheelAt(name string, sampleTime float64) [4]int64 {
	rows, ok := data.eventWheels[name]
	if !ok || len(rows) == 0 {
		return [4]int64{}
	}
	idx := sort.Search(len(rows), func(i int) bool {
		return rows[i].Ts > sampleTime+1e-9
	}) - 1
	if idx < 0 {
		return [4]int64{}
	}
	row := rows[idx]
	return [4]int64{
		int64(math.Round(row.V1)),
		int64(math.Round(row.V2)),
		int64(math.Round(row.V3)),
		int64(math.Round(row.V4)),
	}
}

type channelProfileMapping struct {
	family string
	column string
	mode   string
}

var signalFamilyChannelMappings = map[string]channelProfileMapping{
	"Ground Speed":            {"vehicle_dynamics", "ground_speed", "sampled"},
	"G Force Lat":             {"vehicle_dynamics", "g_force_lat", "sampled"},
	"G Force Long":            {"vehicle_dynamics", "g_force_long", "sampled"},
	"G Force Vert":            {"vehicle_dynamics", "g_force_vert", "sampled"},
	"FFB Output":              {"vehicle_dynamics", "ffb_output", "sampled"},
	"Steering Shaft Torque":   {"vehicle_dynamics", "steering_shaft_torque", "sampled"},
	"Engine RPM":              {"powertrain", "engine_rpm", "sampled"},
	"Clutch RPM":              {"powertrain", "clutch_rpm", "sampled"},
	"Turbo Boost Pressure":    {"powertrain", "turbo_boost_pressure", "sampled"},
	"Engine Max RPM":          {"powertrain", "rpm_max", "sampled"},
	"Throttle Pos":            {"driver_controls", "throttle_pos", "sampled"},
	"Brake Pos":               {"driver_controls", "brake_pos", "sampled"},
	"Steering Pos":            {"driver_controls", "steering_pos", "sampled"},
	"Clutch Pos":              {"driver_controls", "clutch_pos", "sampled"},
	"Throttle Pos Unfiltered": {"driver_controls", "throttle_pos_unfiltered", "sampled"},
	"Brake Pos Unfiltered":    {"driver_controls", "brake_pos_unfiltered", "sampled"},
	"Steering Pos Unfiltered": {"driver_controls", "steering_pos_unfiltered", "sampled"},
	"Clutch Pos Unfiltered":   {"driver_controls", "clutch_pos_unfiltered", "sampled"},
	"Gear":                    {"driver_controls", "gear", "on-change"},
	"ABS":                     {"driver_controls", "abs_active", "on-change"},
	"TC":                      {"driver_controls", "tc_active", "on-change"},
	"Lap":                     {"progress_position", "lap", "on-change"},
	"Current Sector":          {"progress_position", "current_sector", "on-change"},
	"Lap Dist":                {"progress_position", "lap_dist", "sampled"},
	"Total Dist":              {"progress_position", "total_dist", "sampled"},
	"Lap Completion":          {"progress_position", "lap_completion", "sampled"},
	"Path Lateral":            {"progress_position", "path_lateral", "sampled"},
	"Track Edge":              {"progress_position", "track_edge", "sampled"},
	"Position":                {"race_standing", "position", "on-change"},
	"Class Position":          {"race_standing", "class_position", "on-change"},
	"Best LapTime":            {"lap_reference_timing", "best_laptime", "on-change"},
	"Best Sector1":            {"lap_reference_timing", "best_sector1", "on-change"},
	"Best Sector2":            {"lap_reference_timing", "best_sector2", "on-change"},
	"Last LapTime":            {"lap_reference_timing", "last_laptime", "on-change"},
	"Lap Time":                {"lap_reference_timing", "last_laptime", "on-change"},
	"Last Sector1":            {"lap_reference_timing", "last_sector1", "on-change"},
	"Last Sector2":            {"lap_reference_timing", "last_sector2", "on-change"},
	"Delta Best":              {"current_lap_timing", "delta_to_best_lap", "sampled"},
	"Current Sector1":         {"current_lap_timing", "current_sector1", "on-change"},
	"Current Sector2":         {"current_lap_timing", "current_sector2", "on-change"},
	"Current LapTime":         {"source_samples", "current_lap_time", "sampled"},
	"Current Lap Time":        {"source_samples", "current_lap_time", "sampled"},
	"GPS Time":                {"source_samples", "gps_time_raw", "sampled"},
	"Wheel Speed":             {"wheel_dynamics", "wheel_speed", "sampled"},
	"Susp Pos":                {"wheel_dynamics", "suspension_deflection", "sampled"},
	"RideHeights":             {"wheel_dynamics", "ride_height", "sampled"},
	"Brakes Force":            {"wheel_dynamics", "brakes_force", "sampled"},
	"SurfaceTypes":            {"wheel_dynamics", "surface_type", "on-change"},
	"TyresPressure":           {"tyre_state", "tyres_pressure", "sampled"},
	"TyresWear":               {"tyre_state", "tyres_wear", "sampled"},
	"Tyres Wear":              {"tyre_state", "tyres_wear", "sampled"},
	"TyresRubberTemp":         {"tyre_state", "tyres_rubber_temp", "sampled"},
	"TyresTempLeft":           {"tyre_state", "tyres_temp_left", "sampled"},
	"TyresTempCentre":         {"tyre_state", "tyres_temp_centre", "sampled"},
	"TyresTempRight":          {"tyre_state", "tyres_temp_right", "sampled"},
	"TyresCarcassTemp":        {"tyre_state", "carcass_temp", "sampled"},
	"TyresRimTemp":            {"tyre_state", "rim_temp", "sampled"},
	"Brakes Temp":             {"brake_state", "brakes_temp", "sampled"},
	"Brakes Air Temp":         {"brake_state", "brakes_air_temp", "sampled"},
	"Brake Thickness":         {"brake_state", "brake_thickness", "sampled"},
	"FrontRideHeight":         {"aero_platform", "aero_front_ride_height", "sampled"},
	"RearRideHeight":          {"aero_platform", "aero_rear_ride_height", "sampled"},
	"Front3rdDeflection":      {"aero_platform", "aero_front_3rd_deflection", "sampled"},
	"Rear3rdDeflection":       {"aero_platform", "aero_rear_3rd_deflection", "sampled"},
	"Front Wing Height":       {"aero_platform", "front_wing_height", "sampled"},
	"Drag":                    {"aero_platform", "drag", "sampled"},
	"Front Downforce":         {"aero_platform", "front_downforce", "sampled"},
	"Rear Downforce":          {"aero_platform", "rear_downforce", "sampled"},
	"Total Downforce":         {"aero_platform", "total_downforce", "sampled"},
	"Fuel Level":              {"energy_state", "fuel_level", "sampled"},
	"SoC":                     {"energy_state", "battery_soc", "sampled"},
	"Regen Rate":              {"energy_state", "ers_regen_kw", "sampled"},
	"Virtual Energy":          {"energy_state", "ers_virtual_energy", "sampled"},
	"Engine Oil Temp":         {"powertrain_state", "engine_oil_temp", "sampled"},
	"Engine Water Temp":       {"powertrain_state", "engine_water_temp", "sampled"},
	"Motor Temp":              {"powertrain_state", "motor_temp", "sampled"},
	"ERS Water Temp":          {"powertrain_state", "ers_water_temp", "sampled"},
	"Time Behind Next":        {"opponent_context", "opp_ahead_gap_time", "sampled"},
	"Ambient Temperature":     {"environment_state", "air_temp", "sampled"},
	"Track Temperature":       {"environment_state", "track_temp", "sampled"},
	"Wind Speed":              {"environment_state", "wind_speed", "sampled"},
	"Wind Heading":            {"environment_state", "wind_heading", "sampled"},
	"Rain Intensity":          {"environment_state", "rain_intensity", "sampled"},
	"Grip Pct":                {"environment_state", "grip_pct", "sampled"},
	"CloudDarkness":           {"environment_state", "dark_cloud", "on-change"},
	"Minimum Path Wetness":    {"environment_state", "path_wetness_min", "on-change"},
	"OffpathWetness":          {"environment_state", "path_wetness_avg", "on-change"},
	"ABSLevel":                {"car_state", "abs_level", "on-change"},
	"TCLevel":                 {"car_state", "tc_level", "on-change"},
	"TCCut":                   {"car_state", "tc_cut", "on-change"},
	"TCSlipAngle":             {"car_state", "tc_slip", "on-change"},
	"FuelMixtureMap":          {"car_state", "motor_map", "on-change"},
	"Brake Migration":         {"car_state", "migration", "on-change"},
	"RearFlapActivated":       {"car_state", "rear_flap", "on-change"},
	"RearFlapLegalStatus":     {"car_state", "rear_flap_legal", "on-change"},
	"FrontFlapActivated":      {"car_state", "front_flap", "on-change"},
	"Speed Limiter":           {"car_state", "speed_limiter", "on-change"},
	"In Pits":                 {"car_state", "in_pits", "on-change"},
	"Headlights State":        {"car_state", "headlights", "on-change"},
	"OverheatingState":        {"car_state", "overheating", "on-change"},
	"AntiStall Activated":     {"car_state", "anti_stall", "on-change"},
	"Brake Bias Rear":         {"car_state", "rear_brake_bias", "on-change"},
	"LastImpactMagnitude":     {"damage_state", "last_impact_magnitude", "on-change"},
	"WheelsDetached":          {"damage_state", "wheel_detached", "on-change"},
}

var forceLongtailChannels = map[string]bool{
	"GPS Latitude":        true,
	"GPS Longitude":       true,
	"GPS Speed":           true,
	"Clutch RPM":          true,
	"Finish Status":       true,
	"Sector1 Flag":        true,
	"Sector2 Flag":        true,
	"Sector3 Flag":        true,
	"Yellow Flag State":   true,
	"LaunchControlActive": true,
	"TCSlipAngle":         true,
	"OffpathWetness":      true,
}

func (data *importedTelemetry) channelProfiles(sampleRateHz int) []schema.ChannelProfilesRow {
	var rows []schema.ChannelProfilesRow
	for name, mapping := range signalFamilyChannelMappings {
		kind, declaredHz, sampleCount, firstSeq, lastSeq, ok := data.channelProfileStats(name, sampleRateHz)
		if !ok {
			continue
		}
		rows = append(rows, schema.ChannelProfilesRow{
			ChannelName:    name,
			StorageFamily:  mapping.family,
			StorageColumn:  mapping.column,
			Kind:           kind,
			DeclaredHz:     declaredHz,
			Mode:           mapping.mode,
			Source:         SourceLabel,
			ProfileVersion: schema.SignalFamilyProfileVersion,
			SampleCount:    sampleCount,
			EffectiveHz:    declaredHz,
			FirstSampleSeq: firstSeq,
			LastSampleSeq:  lastSeq,
			Quality:        "ok",
			GapCount:       0,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].ChannelName < rows[j].ChannelName
	})
	return rows
}

func (data *importedTelemetry) channelProfileStats(name string, sampleRateHz int) (kind string, declaredHz float64, sampleCount int64, firstSeq int64, lastSeq int64, ok bool) {
	if series, found := data.scalars[name]; found && len(series.values) > 0 {
		return "scalar", float64(series.frequency), int64(len(series.values)), 0, sourceSampleSeq(len(series.values)-1, series.frequency, sampleRateHz), true
	}
	if rows, found := data.events[name]; found && len(rows) > 0 {
		return "event", 1, int64(len(rows)), eventSampleSeq(rows[0].Ts, sampleRateHz), eventSampleSeq(rows[len(rows)-1].Ts, sampleRateHz), true
	}
	if series, found := data.wheels[name]; found && len(series.values) > 0 {
		return "wheel4", float64(series.frequency), int64(len(series.values)), 0, sourceSampleSeq(len(series.values)-1, series.frequency, sampleRateHz), true
	}
	if rows, found := data.eventWheels[name]; found && len(rows) > 0 {
		return "event-wheel4", 1, int64(len(rows)), eventSampleSeq(rows[0].Ts, sampleRateHz), eventSampleSeq(rows[len(rows)-1].Ts, sampleRateHz), true
	}
	return "", 0, 0, 0, 0, false
}

func (data *importedTelemetry) longtailRows(sampleRateHz int) ([]schema.ChannelLongtailScalarRow, []schema.ChannelLongtailWheel4Row, []schema.ChannelLongtailIntRow) {
	var scalarRows []schema.ChannelLongtailScalarRow
	var wheelRows []schema.ChannelLongtailWheel4Row
	var intRows []schema.ChannelLongtailIntRow

	for name, series := range data.scalars {
		if !data.shouldLongtail(name) {
			continue
		}
		for i, value := range series.values {
			scalarRows = append(scalarRows, schema.ChannelLongtailScalarRow{
				ChannelName: name,
				SampleSeq:   sourceSampleSeq(i, series.frequency, sampleRateHz),
				Value:       value,
			})
		}
	}
	for name, rows := range data.events {
		if !data.shouldLongtail(name) {
			continue
		}
		for _, row := range rows {
			seq := eventSampleSeq(row.Ts, sampleRateHz)
			if math.Trunc(row.FloatValue) == row.FloatValue {
				intRows = append(intRows, schema.ChannelLongtailIntRow{ChannelName: name, SampleSeq: seq, Value: int64(row.FloatValue)})
			} else {
				scalarRows = append(scalarRows, schema.ChannelLongtailScalarRow{ChannelName: name, SampleSeq: seq, Value: row.FloatValue})
			}
		}
	}
	for name, series := range data.wheels {
		if !data.shouldLongtail(name) {
			continue
		}
		for i, value := range series.values {
			wheelRows = append(wheelRows, schema.ChannelLongtailWheel4Row{
				ChannelName: name,
				SampleSeq:   sourceSampleSeq(i, series.frequency, sampleRateHz),
				Value1:      value.V1,
				Value2:      value.V2,
				Value3:      value.V3,
				Value4:      value.V4,
			})
		}
	}
	for name, rows := range data.eventWheels {
		if !data.shouldLongtail(name) {
			continue
		}
		for _, value := range rows {
			wheelRows = append(wheelRows, schema.ChannelLongtailWheel4Row{
				ChannelName: name,
				SampleSeq:   eventSampleSeq(value.Ts, sampleRateHz),
				Value1:      value.V1,
				Value2:      value.V2,
				Value3:      value.V3,
				Value4:      value.V4,
			})
		}
	}

	sort.Slice(scalarRows, func(i, j int) bool {
		if scalarRows[i].ChannelName == scalarRows[j].ChannelName {
			return scalarRows[i].SampleSeq < scalarRows[j].SampleSeq
		}
		return scalarRows[i].ChannelName < scalarRows[j].ChannelName
	})
	sort.Slice(wheelRows, func(i, j int) bool {
		if wheelRows[i].ChannelName == wheelRows[j].ChannelName {
			return wheelRows[i].SampleSeq < wheelRows[j].SampleSeq
		}
		return wheelRows[i].ChannelName < wheelRows[j].ChannelName
	})
	sort.Slice(intRows, func(i, j int) bool {
		if intRows[i].ChannelName == intRows[j].ChannelName {
			return intRows[i].SampleSeq < intRows[j].SampleSeq
		}
		return intRows[i].ChannelName < intRows[j].ChannelName
	})

	return scalarRows, wheelRows, intRows
}

func (data *importedTelemetry) shouldLongtail(name string) bool {
	if forceLongtailChannels[name] {
		return true
	}
	_, mapped := signalFamilyChannelMappings[name]
	return !mapped
}

func sourceSampleSeq(index int, frequency int, sampleRateHz int) int64 {
	if frequency <= 0 || sampleRateHz <= 0 {
		return 0
	}
	return int64(math.Floor((float64(index)/float64(frequency))*float64(sampleRateHz) + 1e-9))
}

func eventSampleSeq(ts float64, sampleRateHz int) int64 {
	if sampleRateHz <= 0 || ts <= 0 {
		return 0
	}
	return int64(math.Floor(ts*float64(sampleRateHz) + 1e-9))
}

// importConfig controls import behavior.
type importConfig struct {
	verbose       bool
	dryRun        bool
	writeMetadata bool
	phaseOverride string
}

func defaultImportConfig() importConfig {
	return importConfig{
		writeMetadata: true,
	}
}

// ImportOption configures import behavior.
type ImportOption func(*importConfig)

// WithVerbose enables verbose logging during import.
func WithVerbose(v bool) ImportOption {
	return func(cfg *importConfig) { cfg.verbose = v }
}

// WithDryRun skips writing to the output database.
func WithDryRun(v bool) ImportOption {
	return func(cfg *importConfig) { cfg.dryRun = v }
}

// WithPhaseOverride forces a specific phase (e.g., "practice") instead of
// deriving it from the LMU SessionType metadata.
func WithPhaseOverride(phase string) ImportOption {
	return func(cfg *importConfig) { cfg.phaseOverride = phase }
}
