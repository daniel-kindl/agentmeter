#!/bin/sh
# Single entry point: scan the local session logs, serve the dashboard, and open
# it in a browser. Runs the agentmeter binary sitting beside this file.
set -eu

directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec "$directory/agentmeter" up "$@"
