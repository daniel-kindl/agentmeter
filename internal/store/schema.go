package store

import _ "embed"

// Schema is the SQL used to initialize an agentmeter database.
//
//go:embed schema.sql
var Schema string
