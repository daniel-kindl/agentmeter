// Package config reads agentmeter's optional on-disk configuration.
//
// Configuration is JSON because the project's runtime dependency budget is one
// module, and encoding/json is already in the standard library. Every field is
// optional: a missing or empty file yields the same defaults the binary ships
// with, so the tool stays usable with no configuration at all.
package config
