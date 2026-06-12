package core

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"

	"github.com/La-Pace/lapace-import/internal/schema"
	"github.com/duckdb/duckdb-go/v2"
)

// Writer writes signal-family rows to a SessionStint DuckDB file.
// It holds a *sql.DB for lifecycle management and a pinned *sql.Conn for all
// DDL, metadata, and DuckDB Appender writes.
type Writer struct {
	db   *sql.DB
	conn *sql.Conn
}

// NewWriter creates a new signal-family DuckDB at the given path.
func NewWriter(dbPath string) (*Writer, error) {
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("create Lapace DB %s: %w", dbPath, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping Lapace DB %s: %w", dbPath, err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("pin connection: %w", err)
	}

	if _, err := conn.ExecContext(ctx, `SET threads=4`); err != nil {
		conn.Close()
		db.Close()
		return nil, fmt.Errorf("set threads: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `SET memory_limit='2GB'`); err != nil {
		conn.Close()
		db.Close()
		return nil, fmt.Errorf("set memory_limit: %w", err)
	}

	for _, stmt := range schema.MetadataTableDDL() {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			conn.Close()
			db.Close()
			return nil, fmt.Errorf("create metadata table: %w", err)
		}
	}
	for _, stmt := range schema.CreateSignalFamilyTablesSQL() {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			conn.Close()
			db.Close()
			return nil, fmt.Errorf("create signal-family table: %w", err)
		}
	}
	if _, err := conn.ExecContext(ctx, schema.VersionInsertSQL()); err != nil {
		conn.Close()
		db.Close()
		return nil, fmt.Errorf("insert lapace_version: %w", err)
	}

	return &Writer{db: db, conn: conn}, nil
}

// Close releases the DuckDB connection.
func (w *Writer) Close() error {
	if w.conn != nil {
		w.conn.Close()
	}
	if w.db != nil {
		return w.db.Close()
	}
	return nil
}

// newAppender creates a DuckDB Appender for the given table using the pinned
// connection. The caller must close the returned Appender.
func (w *Writer) newAppender(tableName string) (*duckdb.Appender, error) {
	var appender *duckdb.Appender
	err := w.conn.Raw(func(driverConn any) error {
		dc, ok := driverConn.(driver.Conn)
		if !ok {
			return fmt.Errorf("unexpected driver conn type %T", driverConn)
		}
		var err error
		appender, err = duckdb.NewAppenderFromConn(dc, "", tableName)
		return err
	})
	return appender, err
}

// WriteSessionMetadata writes session metadata to session_metadata.
func (w *Writer) WriteSessionMetadata(sessionID, phase, trackName, vehicleName, vehicleClass, driverName, trackLayout string, recordedAt interface{}) error {
	ctx := context.Background()

	_, err := w.conn.ExecContext(ctx, `
		INSERT OR REPLACE INTO session_metadata
		(session_id, session_type, track_name, vehicle_name, vehicle_class,
		 driver_name, recorded_at, track_layout)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID, phase, trackName, vehicleName, vehicleClass,
		driverName, recordedAt, trackLayout)
	if err != nil {
		return fmt.Errorf("insert session_metadata: %w", err)
	}
	return nil
}

// WriteSignalFamilyRows writes all signal-family rows to the output DB.
func (w *Writer) WriteSignalFamilyRows(ctx context.Context, rows []schema.SignalFamilyRows) (err error) {
	_ = ctx

	appenders, err := w.openCoreAppenders()
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := appenders.close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	for i, row := range rows {
		if err := appenders.append(row); err != nil {
			return fmt.Errorf("append signal-family row %d: %w", i, err)
		}
	}
	return nil
}

type coreAppenders struct {
	sourceSamples      *duckdb.Appender
	driverControls     *duckdb.Appender
	vehicleDynamics    *duckdb.Appender
	powertrain         *duckdb.Appender
	progressPosition   *duckdb.Appender
	wheelDynamics      *duckdb.Appender
	tyreState          *duckdb.Appender
	brakeState         *duckdb.Appender
	aeroPlatform       *duckdb.Appender
	energyState        *duckdb.Appender
	powertrainState    *duckdb.Appender
	raceStanding       *duckdb.Appender
	lapReferenceTiming *duckdb.Appender
	currentLapTiming   *duckdb.Appender
	opponentContext    *duckdb.Appender
	environmentState   *duckdb.Appender
	carState           *duckdb.Appender
	damageState        *duckdb.Appender
	channelProfiles    *duckdb.Appender
	longtailScalar     *duckdb.Appender
	longtailWheel4     *duckdb.Appender
	longtailInt        *duckdb.Appender
	longtailDent8      *duckdb.Appender
	telemetryGaps      *duckdb.Appender
}

func (w *Writer) openCoreAppenders() (*coreAppenders, error) {
	open := func(table string) (*duckdb.Appender, error) {
		appender, err := w.newAppender(table)
		if err != nil {
			return nil, fmt.Errorf("create appender for %s: %w", table, err)
		}
		return appender, nil
	}

	appenders := &coreAppenders{}
	var err error
	if appenders.sourceSamples, err = open("source_samples"); err != nil {
		return nil, err
	}
	if appenders.driverControls, err = open("driver_controls"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.vehicleDynamics, err = open("vehicle_dynamics"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.powertrain, err = open("powertrain"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.progressPosition, err = open("progress_position"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.wheelDynamics, err = open("wheel_dynamics"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.tyreState, err = open("tyre_state"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.brakeState, err = open("brake_state"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.aeroPlatform, err = open("aero_platform"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.energyState, err = open("energy_state"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.powertrainState, err = open("powertrain_state"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.raceStanding, err = open("race_standing"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.lapReferenceTiming, err = open("lap_reference_timing"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.currentLapTiming, err = open("current_lap_timing"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.opponentContext, err = open("opponent_context"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.environmentState, err = open("environment_state"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.carState, err = open("car_state"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.damageState, err = open("damage_state"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.channelProfiles, err = open("channel_profiles"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.longtailScalar, err = open("channel_longtail_scalar"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.longtailWheel4, err = open("channel_longtail_wheel4"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.longtailInt, err = open("channel_longtail_int"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.longtailDent8, err = open("channel_longtail_dent8"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	if appenders.telemetryGaps, err = open("telemetry_gaps"); err != nil {
		_ = appenders.close()
		return nil, err
	}
	return appenders, nil
}

func (a *coreAppenders) close() error {
	var firstErr error
	for _, appender := range []*duckdb.Appender{
		a.sourceSamples,
		a.driverControls,
		a.vehicleDynamics,
		a.powertrain,
		a.progressPosition,
		a.wheelDynamics,
		a.tyreState,
		a.brakeState,
		a.aeroPlatform,
		a.energyState,
		a.powertrainState,
		a.raceStanding,
		a.lapReferenceTiming,
		a.currentLapTiming,
		a.opponentContext,
		a.environmentState,
		a.carState,
		a.damageState,
		a.channelProfiles,
		a.longtailScalar,
		a.longtailWheel4,
		a.longtailInt,
		a.longtailDent8,
		a.telemetryGaps,
	} {
		if appender != nil {
			if err := appender.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (a *coreAppenders) append(rows schema.SignalFamilyRows) error {
	ss := rows.SourceSamples
	if err := a.sourceSamples.AppendRow(
		ss.SampleSeq,
		ss.CaptureTSNS,
		ss.CapturedAt,
		ss.SessionTSRaw,
		ss.CurrentLapTime,
		ss.GPSTimeRaw,
		ss.DeltaTime,
		ss.BatchSeq,
		ss.Source,
	); err != nil {
		return fmt.Errorf("source_samples: %w", err)
	}

	dc := rows.DriverControls
	if err := a.driverControls.AppendRow(
		dc.SampleSeq,
		dc.ThrottlePos,
		dc.BrakePos,
		dc.SteeringPos,
		dc.ClutchPos,
		dc.ThrottlePosUnfiltered,
		dc.BrakePosUnfiltered,
		dc.SteeringPosUnfiltered,
		dc.ClutchPosUnfiltered,
		dc.Gear,
		dc.ABSActive,
		dc.TCActive,
	); err != nil {
		return fmt.Errorf("driver_controls: %w", err)
	}

	vd := rows.VehicleDynamics
	if err := a.vehicleDynamics.AppendRow(
		vd.SampleSeq,
		vd.GroundSpeed,
		vd.GForceLat,
		vd.GForceLong,
		vd.GForceVert,
		vd.GForceMagnitude,
		vd.AccelX,
		vd.AccelY,
		vd.AccelZ,
		vd.PitchVelocity,
		vd.YawVelocity,
		vd.RollVelocity,
		vd.PitchAccel,
		vd.YawAccel,
		vd.RollAccel,
		vd.FFBOutput,
		vd.SteeringShaftTorque,
	); err != nil {
		return fmt.Errorf("vehicle_dynamics: %w", err)
	}

	pt := rows.Powertrain
	if err := a.powertrain.AppendRow(
		pt.SampleSeq,
		pt.EngineRPM,
		pt.MotorRPM,
		pt.ClutchRPM,
		pt.EngineTorque,
		pt.MotorTorque,
		pt.TurboBoostPressure,
		pt.BoostPressureMax,
		pt.RPMMax,
	); err != nil {
		return fmt.Errorf("powertrain: %w", err)
	}

	pp := rows.ProgressPosition
	if err := a.progressPosition.AppendRow(
		pp.SampleSeq,
		pp.Lap,
		pp.CurrentSector,
		pp.LapDist,
		pp.TotalDist,
		pp.LapCompletion,
		pp.PositionX,
		pp.PositionY,
		pp.PositionZ,
		pp.PathLateral,
		pp.TrackEdge,
	); err != nil {
		return fmt.Errorf("progress_position: %w", err)
	}

	wd := rows.WheelDynamics
	if err := a.wheelDynamics.AppendRow(
		wd.SampleSeq,
		wd.WheelSpeed[0],
		wd.WheelSpeed[1],
		wd.WheelSpeed[2],
		wd.WheelSpeed[3],
		wd.SuspensionForce[0],
		wd.SuspensionForce[1],
		wd.SuspensionForce[2],
		wd.SuspensionForce[3],
		wd.SuspensionDeflection[0],
		wd.SuspensionDeflection[1],
		wd.SuspensionDeflection[2],
		wd.SuspensionDeflection[3],
		wd.RideHeight[0],
		wd.RideHeight[1],
		wd.RideHeight[2],
		wd.RideHeight[3],
		wd.TireLoad[0],
		wd.TireLoad[1],
		wd.TireLoad[2],
		wd.TireLoad[3],
		wd.GripFraction[0],
		wd.GripFraction[1],
		wd.GripFraction[2],
		wd.GripFraction[3],
		wd.Camber[0],
		wd.Camber[1],
		wd.Camber[2],
		wd.Camber[3],
		wd.LateralForce[0],
		wd.LateralForce[1],
		wd.LateralForce[2],
		wd.LateralForce[3],
		wd.LongitudinalForce[0],
		wd.LongitudinalForce[1],
		wd.LongitudinalForce[2],
		wd.LongitudinalForce[3],
		wd.LateralSlip[0],
		wd.LateralSlip[1],
		wd.LateralSlip[2],
		wd.LateralSlip[3],
		wd.LongitudinalSlip[0],
		wd.LongitudinalSlip[1],
		wd.LongitudinalSlip[2],
		wd.LongitudinalSlip[3],
		wd.LateralGroundVel[0],
		wd.LateralGroundVel[1],
		wd.LateralGroundVel[2],
		wd.LateralGroundVel[3],
		wd.LongitudinalGroundVel[0],
		wd.LongitudinalGroundVel[1],
		wd.LongitudinalGroundVel[2],
		wd.LongitudinalGroundVel[3],
		wd.BrakePressure[0],
		wd.BrakePressure[1],
		wd.BrakePressure[2],
		wd.BrakePressure[3],
		wd.BrakesForce[0],
		wd.BrakesForce[1],
		wd.BrakesForce[2],
		wd.BrakesForce[3],
		wd.SurfaceType[0],
		wd.SurfaceType[1],
		wd.SurfaceType[2],
		wd.SurfaceType[3],
	); err != nil {
		return fmt.Errorf("wheel_dynamics: %w", err)
	}

	ts := rows.TyreState
	if err := a.tyreState.AppendRow(
		ts.SampleSeq,
		ts.Pressure[0],
		ts.Pressure[1],
		ts.Pressure[2],
		ts.Pressure[3],
		ts.Wear[0],
		ts.Wear[1],
		ts.Wear[2],
		ts.Wear[3],
		ts.RubberTemp[0],
		ts.RubberTemp[1],
		ts.RubberTemp[2],
		ts.RubberTemp[3],
		ts.TempLeft[0],
		ts.TempLeft[1],
		ts.TempLeft[2],
		ts.TempLeft[3],
		ts.TempCentre[0],
		ts.TempCentre[1],
		ts.TempCentre[2],
		ts.TempCentre[3],
		ts.TempRight[0],
		ts.TempRight[1],
		ts.TempRight[2],
		ts.TempRight[3],
		ts.CarcassTemp[0],
		ts.CarcassTemp[1],
		ts.CarcassTemp[2],
		ts.CarcassTemp[3],
		ts.RimTemp[0],
		ts.RimTemp[1],
		ts.RimTemp[2],
		ts.RimTemp[3],
	); err != nil {
		return fmt.Errorf("tyre_state: %w", err)
	}

	bs := rows.BrakeState
	if err := a.brakeState.AppendRow(
		bs.SampleSeq,
		bs.BrakesTemp[0],
		bs.BrakesTemp[1],
		bs.BrakesTemp[2],
		bs.BrakesTemp[3],
		bs.BrakesAirTemp[0],
		bs.BrakesAirTemp[1],
		bs.BrakesAirTemp[2],
		bs.BrakesAirTemp[3],
		bs.BrakeThickness[0],
		bs.BrakeThickness[1],
		bs.BrakeThickness[2],
		bs.BrakeThickness[3],
	); err != nil {
		return fmt.Errorf("brake_state: %w", err)
	}

	ap := rows.AeroPlatform
	if err := a.aeroPlatform.AppendRow(
		ap.SampleSeq,
		ap.AeroFrontRideHeight,
		ap.AeroRearRideHeight,
		ap.FrontWingHeight,
		ap.AeroFront3rdDeflection,
		ap.AeroRear3rdDeflection,
		ap.Drag,
		ap.FrontDownforce,
		ap.RearDownforce,
		ap.TotalDownforce,
	); err != nil {
		return fmt.Errorf("aero_platform: %w", err)
	}

	es := rows.EnergyState
	if err := a.energyState.AppendRow(
		es.SampleSeq,
		es.FuelLevel,
		es.FuelConsumptionRate,
		es.BatterySOC,
		es.ERSRegenKW,
		es.ERSVirtualEnergy,
	); err != nil {
		return fmt.Errorf("energy_state: %w", err)
	}

	ps := rows.PowertrainState
	if err := a.powertrainState.AppendRow(
		ps.SampleSeq,
		ps.EngineOilTemp,
		ps.EngineWaterTemp,
		ps.MotorTemp,
		ps.ERSWaterTemp,
	); err != nil {
		return fmt.Errorf("powertrain_state: %w", err)
	}

	rs := rows.RaceStanding
	if err := a.raceStanding.AppendRow(
		rs.SampleSeq,
		rs.Position,
		rs.ClassPosition,
	); err != nil {
		return fmt.Errorf("race_standing: %w", err)
	}

	lr := rows.LapReferenceTiming
	if err := a.lapReferenceTiming.AppendRow(
		lr.SampleSeq,
		lr.BestLapTime,
		lr.BestSector1,
		lr.BestSector2,
		lr.LastLapTime,
		lr.LastSector1,
		lr.LastSector2,
	); err != nil {
		return fmt.Errorf("lap_reference_timing: %w", err)
	}

	cl := rows.CurrentLapTiming
	if err := a.currentLapTiming.AppendRow(
		cl.SampleSeq,
		cl.DeltaToBestLap,
		cl.CurrentSector1,
		cl.CurrentSector2,
	); err != nil {
		return fmt.Errorf("current_lap_timing: %w", err)
	}

	oc := rows.OpponentContext
	if err := a.opponentContext.AppendRow(
		oc.SampleSeq,
		oc.OppAheadGapTime,
		oc.OppAheadGapDist,
		oc.OppBehindGapTime,
		oc.OppBehindGapDist,
		oc.TimeBehindLeader,
	); err != nil {
		return fmt.Errorf("opponent_context: %w", err)
	}

	env := rows.EnvironmentState
	if err := a.environmentState.AppendRow(
		env.SampleSeq,
		env.RainIntensity,
		env.AirTemp,
		env.TrackTemp,
		env.GripPct,
		env.DarkCloud,
		env.CloudCoverage,
		env.WindSpeed,
		env.WindHeading,
		env.PathWetnessMin,
		env.PathWetnessMax,
		env.PathWetnessAvg,
	); err != nil {
		return fmt.Errorf("environment_state: %w", err)
	}

	cs := rows.CarState
	if err := a.carState.AppendRow(
		cs.SampleSeq,
		cs.ABSLevel,
		cs.ABSMax,
		cs.TCLevel,
		cs.TCMax,
		cs.TCSlip,
		cs.TCSlipMax,
		cs.TCCut,
		cs.TCCutMax,
		cs.MotorMap,
		cs.MotorMapMax,
		cs.Migration,
		cs.MigrationMax,
		cs.FrontARB,
		cs.FrontARBMax,
		cs.RearARB,
		cs.RearARBMax,
		cs.DRSActive,
		cs.DRSAvailable,
		cs.IsPlayer,
		cs.InPits,
		cs.InGarage,
		cs.IsDetached,
		cs.Overheating,
		cs.AntiStall,
		cs.IgnitionStarter,
		cs.FrontFlap,
		cs.RearFlap,
		cs.RearFlapLegal,
		cs.SpeedLimiter,
		cs.SpeedLimiterAvailable,
		cs.Headlights,
		cs.FuelCapacity,
		cs.RearBrakeBias,
		cs.FrontTyreCompoundIndex,
		cs.RearTyreCompoundIndex,
		cs.TyreCompoundType[0],
		cs.TyreCompoundType[1],
		cs.TyreCompoundType[2],
		cs.TyreCompoundType[3],
	); err != nil {
		return fmt.Errorf("car_state: %w", err)
	}

	ds := rows.DamageState
	if err := a.damageState.AppendRow(
		ds.SampleSeq,
		ds.DentSeverity[0],
		ds.DentSeverity[1],
		ds.DentSeverity[2],
		ds.DentSeverity[3],
		ds.DentSeverity[4],
		ds.DentSeverity[5],
		ds.DentSeverity[6],
		ds.DentSeverity[7],
		ds.LastImpactET,
		ds.LastImpactMagnitude,
		ds.LastImpactPosX,
		ds.LastImpactPosY,
		ds.LastImpactPosZ,
		ds.WheelDetached[0],
		ds.WheelDetached[1],
		ds.WheelDetached[2],
		ds.WheelDetached[3],
	); err != nil {
		return fmt.Errorf("damage_state: %w", err)
	}

	for _, profile := range rows.ChannelProfiles {
		if err := a.channelProfiles.AppendRow(
			profile.ChannelName,
			profile.StorageFamily,
			profile.StorageColumn,
			profile.Kind,
			profile.DeclaredHz,
			profile.Mode,
			profile.Source,
			profile.ProfileVersion,
			profile.SampleCount,
			profile.EffectiveHz,
			profile.FirstSampleSeq,
			profile.LastSampleSeq,
			profile.Quality,
			profile.GapCount,
		); err != nil {
			return fmt.Errorf("channel_profiles: %w", err)
		}
	}
	for _, row := range rows.ChannelLongtailScalar {
		if err := a.longtailScalar.AppendRow(row.ChannelName, row.SampleSeq, row.Value); err != nil {
			return fmt.Errorf("channel_longtail_scalar: %w", err)
		}
	}
	for _, row := range rows.ChannelLongtailWheel4 {
		if err := a.longtailWheel4.AppendRow(row.ChannelName, row.SampleSeq, row.Value1, row.Value2, row.Value3, row.Value4); err != nil {
			return fmt.Errorf("channel_longtail_wheel4: %w", err)
		}
	}
	for _, row := range rows.ChannelLongtailInt {
		if err := a.longtailInt.AppendRow(row.ChannelName, row.SampleSeq, row.Value); err != nil {
			return fmt.Errorf("channel_longtail_int: %w", err)
		}
	}
	for _, row := range rows.ChannelLongtailDent8 {
		if err := a.longtailDent8.AppendRow(
			row.ChannelName,
			row.SampleSeq,
			row.Value[0],
			row.Value[1],
			row.Value[2],
			row.Value[3],
			row.Value[4],
			row.Value[5],
			row.Value[6],
			row.Value[7],
		); err != nil {
			return fmt.Errorf("channel_longtail_dent8: %w", err)
		}
	}
	for _, row := range rows.TelemetryGaps {
		if err := a.telemetryGaps.AppendRow(
			row.GapID,
			row.Source,
			row.MissingFromSeq,
			row.MissingToSeq,
			row.Reason,
			row.CreatedAt,
		); err != nil {
			return fmt.Errorf("telemetry_gaps: %w", err)
		}
	}

	return nil
}
