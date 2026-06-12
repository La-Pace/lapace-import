package schema

import (
	"time"

	dbschema "github.com/La-Pace/lapace-core/contract/db/schema"
)

const SignalFamilyProfileVersion = dbschema.SignalFamilyProfileVersion

var SignalFamilyTableNames = append([]string(nil), dbschema.SignalFamilyTables...)

var SignalFamilyTableDDL = map[string]string{
	dbschema.SourceSamplesTable: `CREATE TABLE source_samples (
  sample_seq BIGINT PRIMARY KEY,
  capture_ts_ns BIGINT,
  captured_at TIMESTAMP,
  session_ts_raw DOUBLE,
  current_lap_time DOUBLE,
  gps_time_raw DOUBLE,
  delta_time DOUBLE,
  batch_seq BIGINT,
  source VARCHAR
)`,
	dbschema.DriverControlsTable: `CREATE TABLE driver_controls (
  sample_seq BIGINT PRIMARY KEY,
  throttle_pos DOUBLE,
  brake_pos DOUBLE,
  steering_pos DOUBLE,
  clutch_pos DOUBLE,
  throttle_pos_unfiltered DOUBLE,
  brake_pos_unfiltered DOUBLE,
  steering_pos_unfiltered DOUBLE,
  clutch_pos_unfiltered DOUBLE,
  gear BIGINT,
  abs_active BIGINT,
  tc_active BIGINT
)`,
	dbschema.VehicleDynamicsTable: `CREATE TABLE vehicle_dynamics (
  sample_seq BIGINT PRIMARY KEY,
  ground_speed DOUBLE,
  g_force_lat DOUBLE,
  g_force_long DOUBLE,
  g_force_vert DOUBLE,
  g_force_magnitude DOUBLE,
  accel_x DOUBLE,
  accel_y DOUBLE,
  accel_z DOUBLE,
  pitch_velocity DOUBLE,
  yaw_velocity DOUBLE,
  roll_velocity DOUBLE,
  pitch_accel DOUBLE,
  yaw_accel DOUBLE,
  roll_accel DOUBLE,
  ffb_output DOUBLE,
  steering_shaft_torque DOUBLE
)`,
	dbschema.PowertrainTable: `CREATE TABLE powertrain (
  sample_seq BIGINT PRIMARY KEY,
  engine_rpm DOUBLE,
  motor_rpm DOUBLE,
  clutch_rpm DOUBLE,
  engine_torque DOUBLE,
  motor_torque DOUBLE,
  turbo_boost_pressure DOUBLE,
  boost_pressure_max DOUBLE,
  rpm_max DOUBLE
)`,
	dbschema.ProgressPositionTable: `CREATE TABLE progress_position (
  sample_seq BIGINT PRIMARY KEY,
  lap BIGINT,
  current_sector BIGINT,
  lap_dist DOUBLE,
  total_dist DOUBLE,
  lap_completion DOUBLE,
  position_x DOUBLE,
  position_y DOUBLE,
  position_z DOUBLE,
  path_lateral DOUBLE,
  track_edge DOUBLE
)`,
	dbschema.WheelDynamicsTable: `CREATE TABLE wheel_dynamics (
  sample_seq BIGINT PRIMARY KEY,
  wheel_speed_fl DOUBLE,
  wheel_speed_fr DOUBLE,
  wheel_speed_rl DOUBLE,
  wheel_speed_rr DOUBLE,
  suspension_force_fl DOUBLE,
  suspension_force_fr DOUBLE,
  suspension_force_rl DOUBLE,
  suspension_force_rr DOUBLE,
  suspension_deflection_fl DOUBLE,
  suspension_deflection_fr DOUBLE,
  suspension_deflection_rl DOUBLE,
  suspension_deflection_rr DOUBLE,
  ride_height_fl DOUBLE,
  ride_height_fr DOUBLE,
  ride_height_rl DOUBLE,
  ride_height_rr DOUBLE,
  tire_load_fl DOUBLE,
  tire_load_fr DOUBLE,
  tire_load_rl DOUBLE,
  tire_load_rr DOUBLE,
  grip_fraction_fl DOUBLE,
  grip_fraction_fr DOUBLE,
  grip_fraction_rl DOUBLE,
  grip_fraction_rr DOUBLE,
  camber_fl DOUBLE,
  camber_fr DOUBLE,
  camber_rl DOUBLE,
  camber_rr DOUBLE,
  lateral_force_fl DOUBLE,
  lateral_force_fr DOUBLE,
  lateral_force_rl DOUBLE,
  lateral_force_rr DOUBLE,
  longitudinal_force_fl DOUBLE,
  longitudinal_force_fr DOUBLE,
  longitudinal_force_rl DOUBLE,
  longitudinal_force_rr DOUBLE,
  lateral_slip_fl DOUBLE,
  lateral_slip_fr DOUBLE,
  lateral_slip_rl DOUBLE,
  lateral_slip_rr DOUBLE,
  longitudinal_slip_fl DOUBLE,
  longitudinal_slip_fr DOUBLE,
  longitudinal_slip_rl DOUBLE,
  longitudinal_slip_rr DOUBLE,
  lateral_ground_vel_fl DOUBLE,
  lateral_ground_vel_fr DOUBLE,
  lateral_ground_vel_rl DOUBLE,
  lateral_ground_vel_rr DOUBLE,
  longitudinal_ground_vel_fl DOUBLE,
  longitudinal_ground_vel_fr DOUBLE,
  longitudinal_ground_vel_rl DOUBLE,
  longitudinal_ground_vel_rr DOUBLE,
  brake_pressure_fl DOUBLE,
  brake_pressure_fr DOUBLE,
  brake_pressure_rl DOUBLE,
  brake_pressure_rr DOUBLE,
  brakes_force_fl DOUBLE,
  brakes_force_fr DOUBLE,
  brakes_force_rl DOUBLE,
  brakes_force_rr DOUBLE,
  surface_type_fl BIGINT,
  surface_type_fr BIGINT,
  surface_type_rl BIGINT,
  surface_type_rr BIGINT
)`,
	dbschema.TyreStateTable: `CREATE TABLE tyre_state (
  sample_seq BIGINT PRIMARY KEY,
  tyres_pressure_fl DOUBLE,
  tyres_pressure_fr DOUBLE,
  tyres_pressure_rl DOUBLE,
  tyres_pressure_rr DOUBLE,
  tyres_wear_fl DOUBLE,
  tyres_wear_fr DOUBLE,
  tyres_wear_rl DOUBLE,
  tyres_wear_rr DOUBLE,
  tyres_rubber_temp_fl DOUBLE,
  tyres_rubber_temp_fr DOUBLE,
  tyres_rubber_temp_rl DOUBLE,
  tyres_rubber_temp_rr DOUBLE,
  tyres_temp_left_fl DOUBLE,
  tyres_temp_left_fr DOUBLE,
  tyres_temp_left_rl DOUBLE,
  tyres_temp_left_rr DOUBLE,
  tyres_temp_centre_fl DOUBLE,
  tyres_temp_centre_fr DOUBLE,
  tyres_temp_centre_rl DOUBLE,
  tyres_temp_centre_rr DOUBLE,
  tyres_temp_right_fl DOUBLE,
  tyres_temp_right_fr DOUBLE,
  tyres_temp_right_rl DOUBLE,
  tyres_temp_right_rr DOUBLE,
  carcass_temp_fl DOUBLE,
  carcass_temp_fr DOUBLE,
  carcass_temp_rl DOUBLE,
  carcass_temp_rr DOUBLE,
  rim_temp_fl DOUBLE,
  rim_temp_fr DOUBLE,
  rim_temp_rl DOUBLE,
  rim_temp_rr DOUBLE
)`,
	dbschema.BrakeStateTable: `CREATE TABLE brake_state (
  sample_seq BIGINT PRIMARY KEY,
  brakes_temp_fl DOUBLE,
  brakes_temp_fr DOUBLE,
  brakes_temp_rl DOUBLE,
  brakes_temp_rr DOUBLE,
  brakes_air_temp_fl DOUBLE,
  brakes_air_temp_fr DOUBLE,
  brakes_air_temp_rl DOUBLE,
  brakes_air_temp_rr DOUBLE,
  brake_thickness_fl DOUBLE,
  brake_thickness_fr DOUBLE,
  brake_thickness_rl DOUBLE,
  brake_thickness_rr DOUBLE
)`,
	dbschema.AeroPlatformTable: `CREATE TABLE aero_platform (
  sample_seq BIGINT PRIMARY KEY,
  aero_front_ride_height DOUBLE,
  aero_rear_ride_height DOUBLE,
  front_wing_height DOUBLE,
  aero_front_3rd_deflection DOUBLE,
  aero_rear_3rd_deflection DOUBLE,
  drag DOUBLE,
  front_downforce DOUBLE,
  rear_downforce DOUBLE,
  total_downforce DOUBLE
)`,
	dbschema.EnergyStateTable: `CREATE TABLE energy_state (
  sample_seq BIGINT PRIMARY KEY,
  fuel_level DOUBLE,
  fuel_consumption_rate DOUBLE,
  battery_soc DOUBLE,
  ers_regen_kw DOUBLE,
  ers_virtual_energy DOUBLE
)`,
	dbschema.PowertrainStateTable: `CREATE TABLE powertrain_state (
  sample_seq BIGINT PRIMARY KEY,
  engine_oil_temp DOUBLE,
  engine_water_temp DOUBLE,
  motor_temp DOUBLE,
  ers_water_temp DOUBLE
)`,
	dbschema.RaceStandingTable: `CREATE TABLE race_standing (
  sample_seq BIGINT PRIMARY KEY,
  position BIGINT,
  class_position BIGINT
)`,
	dbschema.LapReferenceTimingTable: `CREATE TABLE lap_reference_timing (
  sample_seq BIGINT PRIMARY KEY,
  best_laptime DOUBLE,
  best_sector1 DOUBLE,
  best_sector2 DOUBLE,
  last_laptime DOUBLE,
  last_sector1 DOUBLE,
  last_sector2 DOUBLE
)`,
	dbschema.CurrentLapTimingTable: `CREATE TABLE current_lap_timing (
  sample_seq BIGINT PRIMARY KEY,
  delta_to_best_lap DOUBLE,
  current_sector1 DOUBLE,
  current_sector2 DOUBLE
)`,
	dbschema.OpponentContextTable: `CREATE TABLE opponent_context (
  sample_seq BIGINT PRIMARY KEY,
  opp_ahead_gap_time DOUBLE,
  opp_ahead_gap_dist DOUBLE,
  opp_behind_gap_time DOUBLE,
  opp_behind_gap_dist DOUBLE,
  time_behind_leader DOUBLE
)`,
	dbschema.EnvironmentStateTable: `CREATE TABLE environment_state (
  sample_seq BIGINT PRIMARY KEY,
  rain_intensity DOUBLE,
  air_temp DOUBLE,
  track_temp DOUBLE,
  grip_pct DOUBLE,
  dark_cloud DOUBLE,
  cloud_coverage DOUBLE,
  wind_speed DOUBLE,
  wind_heading DOUBLE,
  path_wetness_min DOUBLE,
  path_wetness_max DOUBLE,
  path_wetness_avg DOUBLE
)`,
	dbschema.CarStateTable: `CREATE TABLE car_state (
  sample_seq BIGINT PRIMARY KEY,
  abs_level BIGINT,
  abs_max BIGINT,
  tc_level BIGINT,
  tc_max BIGINT,
  tc_slip DOUBLE,
  tc_slip_max DOUBLE,
  tc_cut BIGINT,
  tc_cut_max BIGINT,
  motor_map BIGINT,
  motor_map_max BIGINT,
  migration BIGINT,
  migration_max BIGINT,
  front_arb BIGINT,
  front_arb_max BIGINT,
  rear_arb BIGINT,
  rear_arb_max BIGINT,
  drs_active BIGINT,
  drs_available BIGINT,
  is_player BIGINT,
  in_pits BIGINT,
  in_garage BIGINT,
  is_detached BIGINT,
  overheating BIGINT,
  anti_stall BIGINT,
  ignition_starter BIGINT,
  front_flap DOUBLE,
  rear_flap DOUBLE,
  rear_flap_legal BIGINT,
  speed_limiter BIGINT,
  speed_limiter_available BIGINT,
  headlights BIGINT,
  fuel_capacity DOUBLE,
  rear_brake_bias DOUBLE,
  front_tyre_compound_index BIGINT,
  rear_tyre_compound_index BIGINT,
  tyre_compound_type_fl BIGINT,
  tyre_compound_type_fr BIGINT,
  tyre_compound_type_rl BIGINT,
  tyre_compound_type_rr BIGINT
)`,
	dbschema.StateEventsTable: `CREATE TABLE state_events (
  sample_seq BIGINT PRIMARY KEY,
  event_name VARCHAR,
  value VARCHAR,
  old_value VARCHAR,
  new_value VARCHAR,
  reason VARCHAR
)`,
	dbschema.DamageStateTable: `CREATE TABLE damage_state (
  sample_seq BIGINT PRIMARY KEY,
  dent_severity_1 DOUBLE,
  dent_severity_2 DOUBLE,
  dent_severity_3 DOUBLE,
  dent_severity_4 DOUBLE,
  dent_severity_5 DOUBLE,
  dent_severity_6 DOUBLE,
  dent_severity_7 DOUBLE,
  dent_severity_8 DOUBLE,
  last_impact_et DOUBLE,
  last_impact_magnitude DOUBLE,
  last_impact_pos_x DOUBLE,
  last_impact_pos_y DOUBLE,
  last_impact_pos_z DOUBLE,
  wheel_detached_fl BIGINT,
  wheel_detached_fr BIGINT,
  wheel_detached_rl BIGINT,
  wheel_detached_rr BIGINT
)`,
	dbschema.ChannelProfilesTable: `CREATE TABLE channel_profiles (
  channel_name VARCHAR PRIMARY KEY,
  storage_family VARCHAR,
  storage_column VARCHAR,
  kind VARCHAR,
  declared_hz DOUBLE,
  mode VARCHAR,
  source VARCHAR,
  profile_version INTEGER,
  sample_count BIGINT,
  effective_hz DOUBLE,
  first_sample_seq BIGINT,
  last_sample_seq BIGINT,
  quality VARCHAR,
  gap_count INTEGER
)`,
	// Phase A creates longtail and gap tables for schema stability; batch
	// finalization will populate them when mixed-rate gap tracking lands.
	dbschema.ChannelLongtailScalarTable: `CREATE TABLE channel_longtail_scalar (
  channel_name VARCHAR,
  sample_seq BIGINT,
  value DOUBLE,
  PRIMARY KEY (channel_name, sample_seq)
)`,
	dbschema.ChannelLongtailWheel4Table: `CREATE TABLE channel_longtail_wheel4 (
  channel_name VARCHAR,
  sample_seq BIGINT,
  value1 DOUBLE,
  value2 DOUBLE,
  value3 DOUBLE,
  value4 DOUBLE,
  PRIMARY KEY (channel_name, sample_seq)
)`,
	dbschema.ChannelLongtailIntTable: `CREATE TABLE channel_longtail_int (
  channel_name VARCHAR,
  sample_seq BIGINT,
  value BIGINT,
  PRIMARY KEY (channel_name, sample_seq)
)`,
	dbschema.ChannelLongtailDent8Table: `CREATE TABLE channel_longtail_dent8 (
  channel_name VARCHAR,
  sample_seq BIGINT,
  value1 DOUBLE,
  value2 DOUBLE,
  value3 DOUBLE,
  value4 DOUBLE,
  value5 DOUBLE,
  value6 DOUBLE,
  value7 DOUBLE,
  value8 DOUBLE,
  PRIMARY KEY (channel_name, sample_seq)
)`,
	dbschema.TelemetryGapsTable: `CREATE TABLE telemetry_gaps (
  gap_id VARCHAR PRIMARY KEY,
  source VARCHAR,
  missing_from_seq BIGINT,
  missing_to_seq BIGINT,
  reason VARCHAR,
  created_at TIMESTAMP
)`,
}

func CreateSignalFamilyTablesSQL() []string {
	stmts := make([]string, 0, len(SignalFamilyTableNames))
	for _, name := range SignalFamilyTableNames {
		stmts = append(stmts, SignalFamilyTableDDL[name])
	}
	return stmts
}

type SourceSamplesRow struct {
	SampleSeq      int64
	CaptureTSNS    int64
	CapturedAt     time.Time
	SessionTSRaw   float64
	CurrentLapTime float64
	GPSTimeRaw     float64
	DeltaTime      float64
	BatchSeq       int64
	Source         string
}

type DriverControlsRow struct {
	SampleSeq             int64
	ThrottlePos           float64
	BrakePos              float64
	SteeringPos           float64
	ClutchPos             float64
	ThrottlePosUnfiltered float64
	BrakePosUnfiltered    float64
	SteeringPosUnfiltered float64
	ClutchPosUnfiltered   float64
	Gear                  int64
	ABSActive             int64
	TCActive              int64
}

type VehicleDynamicsRow struct {
	SampleSeq           int64
	GroundSpeed         float64
	GForceLat           float64
	GForceLong          float64
	GForceVert          float64
	GForceMagnitude     float64
	AccelX              float64
	AccelY              float64
	AccelZ              float64
	PitchVelocity       float64
	YawVelocity         float64
	RollVelocity        float64
	PitchAccel          float64
	YawAccel            float64
	RollAccel           float64
	FFBOutput           float64
	SteeringShaftTorque float64
}

type PowertrainRow struct {
	SampleSeq          int64
	EngineRPM          float64
	MotorRPM           float64
	ClutchRPM          float64
	EngineTorque       float64
	MotorTorque        float64
	TurboBoostPressure float64
	BoostPressureMax   float64
	RPMMax             float64
}

type ProgressPositionRow struct {
	SampleSeq     int64
	Lap           int64
	CurrentSector int64
	LapDist       float64
	TotalDist     float64
	LapCompletion float64
	PositionX     float64
	PositionY     float64
	PositionZ     float64
	PathLateral   float64
	TrackEdge     float64
}

type WheelDynamicsRow struct {
	SampleSeq             int64
	WheelSpeed            [4]float64
	SuspensionForce       [4]float64
	SuspensionDeflection  [4]float64
	RideHeight            [4]float64
	TireLoad              [4]float64
	GripFraction          [4]float64
	Camber                [4]float64
	LateralForce          [4]float64
	LongitudinalForce     [4]float64
	LateralSlip           [4]float64
	LongitudinalSlip      [4]float64
	LateralGroundVel      [4]float64
	LongitudinalGroundVel [4]float64
	BrakePressure         [4]float64
	BrakesForce           [4]float64
	SurfaceType           [4]int64
}

type TyreStateRow struct {
	SampleSeq   int64
	Pressure    [4]float64
	Wear        [4]float64
	RubberTemp  [4]float64
	TempLeft    [4]float64
	TempCentre  [4]float64
	TempRight   [4]float64
	CarcassTemp [4]float64
	RimTemp     [4]float64
}

type BrakeStateRow struct {
	SampleSeq      int64
	BrakesTemp     [4]float64
	BrakesAirTemp  [4]float64
	BrakeThickness [4]float64
}

type AeroPlatformRow struct {
	SampleSeq              int64
	AeroFrontRideHeight    float64
	AeroRearRideHeight     float64
	FrontWingHeight        float64
	AeroFront3rdDeflection float64
	AeroRear3rdDeflection  float64
	Drag                   float64
	FrontDownforce         float64
	RearDownforce          float64
	TotalDownforce         float64
}

type EnergyStateRow struct {
	SampleSeq           int64
	FuelLevel           float64
	FuelConsumptionRate float64
	BatterySOC          float64
	ERSRegenKW          float64
	ERSVirtualEnergy    float64
}

type PowertrainStateRow struct {
	SampleSeq       int64
	EngineOilTemp   float64
	EngineWaterTemp float64
	MotorTemp       float64
	ERSWaterTemp    float64
}

type RaceStandingRow struct {
	SampleSeq     int64
	Position      int64
	ClassPosition int64
}

type LapReferenceTimingRow struct {
	SampleSeq   int64
	BestLapTime float64
	BestSector1 float64
	BestSector2 float64
	LastLapTime float64
	LastSector1 float64
	LastSector2 float64
}

type CurrentLapTimingRow struct {
	SampleSeq      int64
	DeltaToBestLap float64
	CurrentSector1 float64
	CurrentSector2 float64
}

type OpponentContextRow struct {
	SampleSeq        int64
	OppAheadGapTime  float64
	OppAheadGapDist  float64
	OppBehindGapTime float64
	OppBehindGapDist float64
	TimeBehindLeader float64
}

type EnvironmentStateRow struct {
	SampleSeq      int64
	RainIntensity  float64
	AirTemp        float64
	TrackTemp      float64
	GripPct        float64
	DarkCloud      float64
	CloudCoverage  float64
	WindSpeed      float64
	WindHeading    float64
	PathWetnessMin float64
	PathWetnessMax float64
	PathWetnessAvg float64
}

type CarStateRow struct {
	SampleSeq              int64
	ABSLevel               int64
	ABSMax                 int64
	TCLevel                int64
	TCMax                  int64
	TCSlip                 float64
	TCSlipMax              float64
	TCCut                  int64
	TCCutMax               int64
	MotorMap               int64
	MotorMapMax            int64
	Migration              int64
	MigrationMax           int64
	FrontARB               int64
	FrontARBMax            int64
	RearARB                int64
	RearARBMax             int64
	DRSActive              int64
	DRSAvailable           int64
	IsPlayer               int64
	InPits                 int64
	InGarage               int64
	IsDetached             int64
	Overheating            int64
	AntiStall              int64
	IgnitionStarter        int64
	FrontFlap              float64
	RearFlap               float64
	RearFlapLegal          int64
	SpeedLimiter           int64
	SpeedLimiterAvailable  int64
	Headlights             int64
	FuelCapacity           float64
	RearBrakeBias          float64
	FrontTyreCompoundIndex int64
	RearTyreCompoundIndex  int64
	TyreCompoundType       [4]int64
}

type StateEventsRow struct {
	SampleSeq int64
	EventName string
	Value     string
	OldValue  string
	NewValue  string
	Reason    string
}

type DamageStateRow struct {
	SampleSeq           int64
	DentSeverity        [8]float64
	LastImpactET        float64
	LastImpactMagnitude float64
	LastImpactPosX      float64
	LastImpactPosY      float64
	LastImpactPosZ      float64
	WheelDetached       [4]int64
}

type ChannelProfilesRow struct {
	ChannelName    string
	StorageFamily  string
	StorageColumn  string
	Kind           string
	DeclaredHz     float64
	Mode           string
	Source         string
	ProfileVersion int32
	SampleCount    int64
	EffectiveHz    float64
	FirstSampleSeq int64
	LastSampleSeq  int64
	Quality        string
	GapCount       int32
}

type ChannelLongtailScalarRow struct {
	ChannelName string
	SampleSeq   int64
	Value       float64
}
type ChannelLongtailWheel4Row struct {
	ChannelName string
	SampleSeq   int64
	Value1      float64
	Value2      float64
	Value3      float64
	Value4      float64
}
type ChannelLongtailIntRow struct {
	ChannelName string
	SampleSeq   int64
	Value       int64
}
type ChannelLongtailDent8Row struct {
	ChannelName string
	SampleSeq   int64
	Value       [8]float64
}
type TelemetryGapsRow struct {
	GapID          string
	Source         string
	MissingFromSeq int64
	MissingToSeq   int64
	Reason         string
	CreatedAt      time.Time
}

type SignalFamilyRows struct {
	SourceSamples         SourceSamplesRow
	DriverControls        DriverControlsRow
	VehicleDynamics       VehicleDynamicsRow
	Powertrain            PowertrainRow
	ProgressPosition      ProgressPositionRow
	WheelDynamics         WheelDynamicsRow
	TyreState             TyreStateRow
	BrakeState            BrakeStateRow
	AeroPlatform          AeroPlatformRow
	EnergyState           EnergyStateRow
	PowertrainState       PowertrainStateRow
	RaceStanding          RaceStandingRow
	LapReferenceTiming    LapReferenceTimingRow
	CurrentLapTiming      CurrentLapTimingRow
	OpponentContext       OpponentContextRow
	EnvironmentState      EnvironmentStateRow
	CarState              CarStateRow
	StateEvents           StateEventsRow
	DamageState           DamageStateRow
	ChannelProfiles       []ChannelProfilesRow
	ChannelLongtailScalar []ChannelLongtailScalarRow
	ChannelLongtailWheel4 []ChannelLongtailWheel4Row
	ChannelLongtailInt    []ChannelLongtailIntRow
	ChannelLongtailDent8  []ChannelLongtailDent8Row
	TelemetryGaps         []TelemetryGapsRow
}
