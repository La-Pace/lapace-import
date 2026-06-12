package schema

import (
	"fmt"

	dbschema "github.com/La-Pace/lapace-core/contract/db/schema"
	telemetry "github.com/La-Pace/lapace-core/contract/telemetry"
)

const SignalFamilySchemaType = dbschema.SignalFamilySchemaType

var (
	LapaceVersionTableDDL = fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s (%s VARCHAR PRIMARY KEY, %s INTEGER, %s VARCHAR, %s TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`,
		dbschema.LapaceVersionTable,
		dbschema.ColumnSchemaVersion,
		dbschema.ColumnDataVersion,
		dbschema.ColumnSchemaType,
		dbschema.ColumnCreatedAt,
	)
	SessionMetadataTableDDL = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
  %s VARCHAR PRIMARY KEY,
  %s VARCHAR,
  %s VARCHAR,
  %s VARCHAR,
  %s VARCHAR,
  %s VARCHAR,
  %s TIMESTAMP,
  %s BIGINT,
  %s DOUBLE,
  %s DOUBLE,
  %s VARCHAR,
  %s DOUBLE
)`,
		dbschema.SessionMetadataTable,
		dbschema.ColumnSessionID,
		dbschema.ColumnSessionType,
		dbschema.ColumnTrackName,
		dbschema.ColumnVehicleName,
		dbschema.ColumnVehicleClass,
		dbschema.ColumnDriverName,
		dbschema.ColumnRecordedAt,
		dbschema.ColumnFrameCount,
		dbschema.ColumnDuration,
		dbschema.ColumnSampleRateHz,
		dbschema.ColumnTrackLayout,
		dbschema.ColumnTrackLength,
	)
)

func MetadataTableDDL() map[string]string {
	return map[string]string{
		dbschema.LapaceVersionTable:   LapaceVersionTableDDL,
		dbschema.SessionMetadataTable: SessionMetadataTableDDL,
	}
}

// VersionInsertSQL returns the SQL to insert the current signal-family schema version.
func VersionInsertSQL() string {
	return fmt.Sprintf(
		"INSERT OR REPLACE INTO %s (%s, %s, %s) VALUES ('%s', %d, '%s')",
		dbschema.LapaceVersionTable,
		dbschema.ColumnSchemaVersion,
		dbschema.ColumnDataVersion,
		dbschema.ColumnSchemaType,
		telemetry.SchemaVersion, telemetry.DataVersion, SignalFamilySchemaType,
	)
}
