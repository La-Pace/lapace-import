package lmu

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/duckdb/duckdb-go/v2"
)

// LMUFile represents an opened official LMU DuckDB telemetry export.
type LMUFile struct {
	db *sql.DB
}

// LMUMetadata holds the key-value metadata from an official LMU DuckDB export.
type LMUMetadata struct {
	Version           string
	DriverName        string
	SteamID           string
	RecordingTime     string
	SessionTime       string
	SessionType       string
	TrackName         string
	TrackLayout       string
	WeatherConditions string
	CarName           string
	CarClass          string
	CarSetup          string // raw JSON blob, may be empty
}

// OpenLMUFile opens an official LMU DuckDB export file in read-only mode.
func OpenLMUFile(path string) (*LMUFile, error) {
	dsn := path + "?access_mode=READ_ONLY"
	db, err := sql.Open("duckdb", dsn)
	if err != nil {
		return nil, fmt.Errorf("open LMU DuckDB %s: %w", path, err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping LMU DuckDB %s: %w", path, err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	return &LMUFile{db: db}, nil
}

// Close releases the DuckDB connection.
func (f *LMUFile) Close() error {
	if f.db != nil {
		return f.db.Close()
	}
	return nil
}

// EventInfo holds event channel metadata from the eventsList table.
type EventInfo struct {
	Name string
	Unit string
}

// ChannelInfo holds channel metadata from the channelsList table.
type ChannelInfo struct {
	Name      string
	Frequency int
	Unit      string
}

// ScalarRow holds a single row from a scalar channel table (no ts column).
type ScalarRow struct {
	Value float64
}

// EventRow holds a single row from an event/state channel table (has ts column).
// The value column type varies across LMU event tables (TINYINT, SMALLINT,
// USMALLINT, UINTEGER, BOOLEAN). We store the raw numeric value as float64
// for uniform handling; boolean TRUE maps to 1.0, FALSE to 0.0.
type EventRow struct {
	Ts         float64
	FloatValue float64
}

// WheelRow holds a single row from a per-wheel channel table (no ts column).
type WheelRow struct {
	V1 float64 // FL
	V2 float64 // FR
	V3 float64 // RL
	V4 float64 // RR
}

// Metadata reads the metadata key-value table from the LMU DuckDB export.
func (f *LMUFile) Metadata() (*LMUMetadata, error) {
	ctx := context.Background()

	rows, err := f.db.QueryContext(ctx, `SELECT key, value FROM metadata ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}
	defer rows.Close()

	kv := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan metadata row: %w", err)
		}
		kv[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("metadata rows: %w", err)
	}

	meta := &LMUMetadata{
		Version:           kv["Version"],
		DriverName:        kv["DriverName"],
		SteamID:           kv["SteamID"],
		RecordingTime:     kv["RecordingTime"],
		SessionTime:       kv["SessionTime"],
		SessionType:       kv["SessionType"],
		TrackName:         kv["TrackName"],
		TrackLayout:       kv["TrackLayout"],
		WeatherConditions: kv["WeatherConditions"],
		CarName:           kv["CarName"],
		CarClass:          kv["CarClass"],
		CarSetup:          kv["CarSetup"],
	}

	return meta, nil
}

// ChannelsList reads the channelsList table from the LMU DuckDB export.
func (f *LMUFile) ChannelsList() ([]ChannelInfo, error) {
	ctx := context.Background()

	rows, err := f.db.QueryContext(ctx, `SELECT channelName, frequency, unit FROM channelsList ORDER BY channelName`)
	if err != nil {
		return nil, fmt.Errorf("read channelsList: %w", err)
	}
	defer rows.Close()

	var channels []ChannelInfo
	for rows.Next() {
		var ch ChannelInfo
		if err := rows.Scan(&ch.Name, &ch.Frequency, &ch.Unit); err != nil {
			return nil, fmt.Errorf("scan channelsList row: %w", err)
		}
		channels = append(channels, ch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("channelsList rows: %w", err)
	}

	return channels, nil
}

// EventsList reads the eventsList table from the LMU DuckDB export.
func (f *LMUFile) EventsList() ([]EventInfo, error) {
	ctx := context.Background()

	rows, err := f.db.QueryContext(ctx, `SELECT eventName, unit FROM eventsList ORDER BY eventName`)
	if err != nil {
		return nil, fmt.Errorf("read eventsList: %w", err)
	}
	defer rows.Close()

	var events []EventInfo
	for rows.Next() {
		var ev EventInfo
		if err := rows.Scan(&ev.Name, &ev.Unit); err != nil {
			return nil, fmt.Errorf("scan eventsList row: %w", err)
		}
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("eventsList rows: %w", err)
	}

	return events, nil
}

// ChannelTables returns a list of all non-metadata table names in the LMU DuckDB export.
// These are the data tables containing telemetry channels.
func (f *LMUFile) ChannelTables() ([]string, error) {
	ctx := context.Background()

	rows, err := f.db.QueryContext(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'main'
		ORDER BY table_name
	`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan table name: %w", err)
		}
		// Exclude metadata tables
		if name == "metadata" || name == "channelsList" || name == "eventsList" {
			continue
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("table rows: %w", err)
	}

	return tables, nil
}

// ReadScalarTable reads all rows from a scalar channel table.
// LMU scalar tables may have value columns of type FLOAT, DOUBLE, BOOLEAN, or
// integer types. We CAST to DOUBLE for uniform handling.
func (f *LMUFile) ReadScalarTable(ctx context.Context, name string) ([]ScalarRow, error) {
	// Pre-allocate using COUNT(*)
	var count int
	if err := f.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, name)).Scan(&count); err != nil {
		return nil, fmt.Errorf("count scalar table %q: %w", name, err)
	}

	query := fmt.Sprintf(`SELECT CAST("value" AS DOUBLE) FROM "%s" ORDER BY rowid`, name)
	rows, err := f.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("read scalar table %q: %w", name, err)
	}
	defer rows.Close()

	result := make([]ScalarRow, 0, count)
	for rows.Next() {
		var r ScalarRow
		if err := rows.Scan(&r.Value); err != nil {
			return nil, fmt.Errorf("scan scalar row in %q: %w", name, err)
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scalar rows in %q: %w", name, err)
	}

	return result, nil
}

// ReadEventTable reads all rows from an event/state channel table (ts DOUBLE, value <type>).
// Values are normalized to float64 (booleans map to 1.0/0.0).
func (f *LMUFile) ReadEventTable(ctx context.Context, name string) ([]EventRow, error) {
	// Pre-allocate using COUNT(*)
	var count int
	if err := f.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, name)).Scan(&count); err != nil {
		return nil, fmt.Errorf("count event table %q: %w", name, err)
	}

	query := fmt.Sprintf(`SELECT ts, CAST(value AS DOUBLE) FROM "%s" ORDER BY ts`, name)
	rows, err := f.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("read event table %q: %w", name, err)
	}
	defer rows.Close()

	result := make([]EventRow, 0, count)
	for rows.Next() {
		var r EventRow
		if err := rows.Scan(&r.Ts, &r.FloatValue); err != nil {
			return nil, fmt.Errorf("scan event row in %q: %w", name, err)
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("event rows in %q: %w", name, err)
	}

	return result, nil
}

// EventWheelRow holds a single row from an event-wheel channel table (ts DOUBLE, value1-4 <type>).
// Like WheelRow but with a timestamp column.
type EventWheelRow struct {
	Ts float64
	V1 float64 // FL
	V2 float64 // FR
	V3 float64 // RL
	V4 float64 // RR
}

// ReadWheelTable reads all rows from a per-wheel channel table.
// Values are CAST to DOUBLE for uniform handling (same pattern as scalar/event).
func (f *LMUFile) ReadWheelTable(ctx context.Context, name string) ([]WheelRow, error) {
	// Pre-allocate using COUNT(*)
	var count int
	if err := f.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, name)).Scan(&count); err != nil {
		return nil, fmt.Errorf("count wheel table %q: %w", name, err)
	}

	query := fmt.Sprintf(`SELECT CAST(value1 AS DOUBLE), CAST(value2 AS DOUBLE), CAST(value3 AS DOUBLE), CAST(value4 AS DOUBLE) FROM "%s" ORDER BY rowid`, name)
	rows, err := f.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("read wheel table %q: %w", name, err)
	}
	defer rows.Close()

	result := make([]WheelRow, 0, count)
	for rows.Next() {
		var r WheelRow
		if err := rows.Scan(&r.V1, &r.V2, &r.V3, &r.V4); err != nil {
			return nil, fmt.Errorf("scan wheel row in %q: %w", name, err)
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("wheel rows in %q: %w", name, err)
	}

	return result, nil
}

// ReadEventWheelTable reads all rows from an event-wheel channel table (ts DOUBLE, value1-4 <type>).
// Values are normalized to float64, consistent with event channel handling.
func (f *LMUFile) ReadEventWheelTable(ctx context.Context, name string) ([]EventWheelRow, error) {
	// Pre-allocate using COUNT(*)
	var count int
	if err := f.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, name)).Scan(&count); err != nil {
		return nil, fmt.Errorf("count event-wheel table %q: %w", name, err)
	}

	query := fmt.Sprintf(`SELECT ts, CAST(value1 AS DOUBLE), CAST(value2 AS DOUBLE), CAST(value3 AS DOUBLE), CAST(value4 AS DOUBLE) FROM "%s" ORDER BY ts`, name)
	rows, err := f.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("read event-wheel table %q: %w", name, err)
	}
	defer rows.Close()

	result := make([]EventWheelRow, 0, count)
	for rows.Next() {
		var r EventWheelRow
		if err := rows.Scan(&r.Ts, &r.V1, &r.V2, &r.V3, &r.V4); err != nil {
			return nil, fmt.Errorf("scan event-wheel row in %q: %w", name, err)
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("event-wheel rows in %q: %w", name, err)
	}

	return result, nil
}
