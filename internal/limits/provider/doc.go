// Package provider fetches authoritative limit windows from Claude and Codex.
//
// Everything here is opt-in. It reads local agent credentials and contacts the
// vendors' undocumented usage endpoints, which the rest of agentmeter never
// does, so nothing in this package runs unless the operator asks for it.
package provider
