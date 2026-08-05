package main

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "version", args: []string{"version"}, wantCode: 0, wantStdout: "test-version\n"},
		{name: "scan", args: []string{"scan"}, wantCode: 0, wantStdout: "scan is not implemented\n"},
		{name: "missing", wantCode: 2, wantStderr: usageText},
		{name: "unknown", args: []string{"other"}, wantCode: 2, wantStderr: `unknown command "other"`},
		{name: "unexpected argument", args: []string{"scan", "extra"}, wantCode: 2, wantStderr: `unexpected argument "extra"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			app := application{
				stdout:  &stdout,
				stderr:  &stderr,
				version: "test-version",
				listenAndServe: func(string, http.Handler) error {
					t.Fatal("listenAndServe called unexpectedly")
					return nil
				},
			}

			if got := app.run(tt.args); got != tt.wantCode {
				t.Errorf("exit code = %d, want %d", got, tt.wantCode)
			}
			if got := stdout.String(); got != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", got, tt.wantStdout)
			}
			if got := stderr.String(); !strings.Contains(got, tt.wantStderr) {
				t.Errorf("stderr = %q, want substring %q", got, tt.wantStderr)
			}
		})
	}
}

func TestRunServe(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var gotAddress string
	var gotHandler http.Handler
	app := application{
		stdout:  &stdout,
		stderr:  &stderr,
		version: "test-version",
		listenAndServe: func(address string, handler http.Handler) error {
			gotAddress = address
			gotHandler = handler
			return nil
		},
	}

	if got := app.run([]string{"serve"}); got != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", got, stderr.String())
	}
	if gotAddress != serveAddress {
		t.Errorf("address = %q, want %q", gotAddress, serveAddress)
	}
	if gotHandler == nil {
		t.Fatal("handler is nil")
	}
	recorder := httptest.NewRecorder()
	gotHandler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK {
		t.Errorf("handler status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestRunServeError(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := application{
		stdout:  &stdout,
		stderr:  &stderr,
		version: "test-version",
		listenAndServe: func(string, http.Handler) error {
			return errors.New("synthetic listen failure")
		},
	}

	if got := app.run([]string{"serve"}); got != 1 {
		t.Fatalf("exit code = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "synthetic listen failure") {
		t.Errorf("stderr = %q, want listen failure", stderr.String())
	}
}
