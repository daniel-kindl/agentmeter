package store

import _ "embed"

// SchemaVersion is the newest SQLite schema version understood by this binary.
const SchemaVersion = 1

// Schema is the SQL used to initialize an agentmeter database.
//
//go:embed schema.sql
var Schema string
