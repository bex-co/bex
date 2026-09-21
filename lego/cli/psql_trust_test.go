package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The database's external connection string pins sslmode=verify-full against
// the CNPG cluster's private CA (lego/backend/internal/postgres/service.go), so
// a stock client holds no trust root for it. These tests drive the real
// launcher binary against a stub bex-api and a stub `psql` that reports the
// trust material it was handed.
const (
	trustDatabaseID  = "dpg-d3k9q2s1f7v4h8n0m2ab"
	trustServerCAPEM = "-----BEGIN CERTIFICATE-----\nbex-cnpg-test-ca\n-----END CERTIFICATE-----\n"
	trustConnection  = "postgresql://u:p@db.example.test:5432/d?sslmode=verify-full"
)

// psqlTrustStub serves the two reads a `bex psql <id> -c …` makes: the database
// itself (for the client-side allow-list gate) and its connection info.
func psqlTrustStub(t *testing.T, connectionStatus int, connectionBody string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/postgres/" + trustDatabaseID:
			_, _ = fmt.Fprintf(w, `{"id":%q,"name":"qa-trust-db","ipAllowList":[{"cidrBlock":"0.0.0.0/0","description":"test"}]}`, trustDatabaseID)
		case "/v1/postgres/" + trustDatabaseID + "/connection-info":
			w.WriteHeader(connectionStatus)
			_, _ = w.Write([]byte(connectionBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func connectionInfoWithCA(t *testing.T) string {
	t.Helper()
	body, err := json.Marshal(map[string]string{
		"password":                 "p",
		"externalConnectionString": trustConnection,
		"serverCaCertificate":      trustServerCAPEM,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// stubPSQL puts a `psql` on PATH that reports the trust root it inherited and
// records that it ran at all. It returns the directory to prepend to PATH and
// the path of the ran-marker.
func stubPSQL(t *testing.T) (string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub psql is a POSIX shell script")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "psql-ran")
	script := "#!/bin/sh\n" +
		"touch " + marker + "\n" +
		`echo "PGSSLROOTCERT=${PGSSLROOTCERT:-unset}"` + "\n" +
		`if [ -n "${PGSSLROOTCERT:-}" ] && [ -f "${PGSSLROOTCERT}" ]; then cat "${PGSSLROOTCERT}"; fi` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "psql"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return dir, marker
}

func runBexPSQL(t *testing.T, api *httptest.Server, stubDir, home string, extraEnv ...string) (string, error) {
	t.Helper()
	command := exec.Command(buildBex(), "psql", trustDatabaseID, "-c", "SELECT 1", "-o", "text")
	command.Env = append(withoutRenderEnv(os.Environ()),
		"HOME="+home,
		"PATH="+stubDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"BEX_HOST="+api.URL+"/v1/",
		"BEX_ACCESS_TOKEN=test-access-token",
		"BEX_NO_UPDATE_NOTIFIER=1",
	)
	command.Env = append(command.Env, extraEnv...)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	return output.String(), err
}

// Without launcher-side provisioning this is the filed bug: psql is handed a
// verify-full URL and no CA, and fails before any SQL runs.
func TestBexPSQLProvisionsTheServerCAForTheChildProcess(t *testing.T) {
	api := psqlTrustStub(t, http.StatusOK, connectionInfoWithCA(t))
	stubDir, marker := stubPSQL(t)

	output, err := runBexPSQL(t, api, stubDir, t.TempDir())
	if err != nil {
		t.Fatalf("bex psql: %v\n%s", err, output)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("psql never ran: %v\n%s", err, output)
	}
	if !strings.Contains(output, "bex-cnpg-test-ca") {
		t.Fatalf("psql was not handed the database's server CA:\n%s", output)
	}

	// The temporary trust root is the launcher's own and must not survive it.
	path := ""
	for _, line := range strings.Split(output, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "PGSSLROOTCERT="); ok {
			path = rest
		}
	}
	if path == "" || path == "unset" {
		t.Fatalf("PGSSLROOTCERT was not reported by the child:\n%s", output)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("temporary CA %q outlived the command: %v", path, err)
	}
}

// A user who already points PGSSLROOTCERT somewhere keeps that choice.
func TestBexPSQLKeepsAUserProvidedPGSSLROOTCERT(t *testing.T) {
	api := psqlTrustStub(t, http.StatusOK, connectionInfoWithCA(t))
	stubDir, _ := stubPSQL(t)

	home := t.TempDir()
	userCA := filepath.Join(home, "my-own-ca.crt")
	if err := os.WriteFile(userCA, []byte("-----BEGIN CERTIFICATE-----\nuser-chosen-ca\n-----END CERTIFICATE-----\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	output, err := runBexPSQL(t, api, stubDir, home, "PGSSLROOTCERT="+userCA)
	if err != nil {
		t.Fatalf("bex psql: %v\n%s", err, output)
	}
	if !strings.Contains(output, "PGSSLROOTCERT="+userCA) {
		t.Fatalf("the user's PGSSLROOTCERT was overridden:\n%s", output)
	}
	if !strings.Contains(output, "user-chosen-ca") || strings.Contains(output, "bex-cnpg-test-ca") {
		t.Fatalf("psql read the launcher's CA instead of the user's:\n%s", output)
	}
}

// When bex-api cannot serve the CA (503), the user must get a message that
// names the cause rather than psql's opaque certificate error.
func TestBexPSQLReportsAnUnavailableServerCA(t *testing.T) {
	api := psqlTrustStub(t, http.StatusServiceUnavailable,
		`{"error":"service unavailable: Postgres \"`+trustDatabaseID+`\" server CA is not provisioned yet; retry shortly",`+
			`"message":"service unavailable: Postgres \"`+trustDatabaseID+`\" server CA is not provisioned yet; retry shortly","id":"e-503"}`)
	stubDir, marker := stubPSQL(t)

	output, err := runBexPSQL(t, api, stubDir, t.TempDir())
	if err == nil {
		t.Fatalf("bex psql succeeded without a trust root:\n%s", output)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("psql was launched even though it could not possibly connect:\n%s", output)
	}
	for _, want := range []string{"server CA", "verify-full", trustDatabaseID} {
		if !strings.Contains(output, want) {
			t.Errorf("refusal does not mention %q:\n%s", want, output)
		}
	}
}
