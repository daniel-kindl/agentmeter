package store

import (
	// Register the pure-Go SQLite implementation with database/sql.
	_ "modernc.org/sqlite"
)

// DriverName is the database/sql name registered by modernc.org/sqlite.
const DriverName = "sqlite"
