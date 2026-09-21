package pgtrust

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

const testCertificate = "-----BEGIN CERTIFICATE-----\ntest-cnpg-ca\n-----END CERTIFICATE-----\n"

// psqlArity mirrors the flags the pinned upstream `psql`/`pgcli` commands
// define: the global output flag and psql's value-taking --command.
func psqlArity() FlagArity {
	set := pflag.NewFlagSet("psql", pflag.ContinueOnError)
	set.StringP("output", "o", "", "output format")
	set.StringP("command", "c", "", "SQL")
	set.Bool("confirm", false, "skip confirmation")
	return ArityOf(set)
}

// options returns a Provision setup whose every process dependency is local to
// the test: an empty environment, a home directory with no libpq trust store,
// and a temp dir the CA is written into.
func options(t *testing.T, tool string, args []string, fetch func(context.Context, string) (string, error)) (Options, map[string]string, string) {
	t.Helper()
	env := map[string]string{}
	home := t.TempDir()
	return Options{
		Tool:  tool,
		Args:  args,
		Arity: psqlArity(),
		Fetch: fetch,
		Lookup: func(key string) (string, bool) {
			value, ok := env[key]
			return value, ok
		},
		Setenv: func(key, value string) error {
			env[key] = value
			return nil
		},
		Home: func() (string, error) { return home, nil },
		Dir:  t.TempDir(),
	}, env, home
}

func servesCA(certificate string) func(context.Context, string) (string, error) {
	return func(context.Context, string) (string, error) { return certificate, nil }
}

func refuses(t *testing.T) func(context.Context, string) (string, error) {
	t.Helper()
	return func(context.Context, string) (string, error) {
		t.Error("connection-info was read even though the user already has a trust root")
		return "", nil
	}
}

// The bug this closes: a stock client holds no CNPG CA, so the verify-full
// connection string fails before any SQL runs. The launcher must hand the
// child a trust root.
func TestProvisionPointsPGSSLROOTCERTAtTheServerCA(t *testing.T) {
	for _, tool := range []string{"psql", "pgcli"} {
		t.Run(tool, func(t *testing.T) {
			opts, env, _ := options(t, tool,
				[]string{"dpg-d0000000000000000000ab", "-c", "SELECT 1"}, servesCA(testCertificate))

			release, err := Provision(context.Background(), opts)
			if err != nil {
				t.Fatalf("Provision: %v", err)
			}

			path := env[RootCertEnv]
			if path == "" {
				t.Fatal("PGSSLROOTCERT was not set, so the child still has no trust root")
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read provisioned CA: %v", err)
			}
			if string(content) != testCertificate {
				t.Fatalf("provisioned CA = %q, want the server CA", content)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if mode := info.Mode().Perm(); mode != 0o600 {
				t.Fatalf("CA file mode = %04o, want 0600", mode)
			}

			release()
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("temporary CA outlived the session: %v", err)
			}
		})
	}
}

// The selector can sit after flags, and psql's -c consumes its value — a value
// that must never be mistaken for the database.
func TestProvisionFindsTheDatabaseBehindFlags(t *testing.T) {
	var asked string
	opts, env, _ := options(t, "psql",
		[]string{"-o", "text", "-c", "SELECT 1", "dpg-d0000000000000000000ab", "--", "--csv"},
		func(_ context.Context, idOrName string) (string, error) {
			asked = idOrName
			return testCertificate, nil
		})

	if _, err := Provision(context.Background(), opts); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if asked != "dpg-d0000000000000000000ab" {
		t.Fatalf("resolved database = %q, want the positional selector", asked)
	}
	if env[RootCertEnv] == "" {
		t.Fatal("PGSSLROOTCERT was not set")
	}
}

// An explicit user choice is never overridden.
func TestProvisionRespectsUserSetPGSSLROOTCERT(t *testing.T) {
	opts, env, _ := options(t, "psql", []string{"dpg-d0000000000000000000ab"}, refuses(t))
	env[RootCertEnv] = "/home/user/my-ca.crt"

	release, err := Provision(context.Background(), opts)
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	release()
	if env[RootCertEnv] != "/home/user/my-ca.crt" {
		t.Fatalf("PGSSLROOTCERT = %q, want the user's own value untouched", env[RootCertEnv])
	}
}

// libpq's implicit per-user trust store means the user curates trust already.
func TestProvisionRespectsExistingUserRootCert(t *testing.T) {
	opts, env, home := options(t, "psql", []string{"dpg-d0000000000000000000ab"}, refuses(t))
	if err := os.MkdirAll(filepath.Join(home, ".postgresql"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".postgresql", "root.crt"), []byte(testCertificate), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Provision(context.Background(), opts); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if _, set := env[RootCertEnv]; set {
		t.Fatalf("PGSSLROOTCERT was set to %q, shadowing the user's own trust store", env[RootCertEnv])
	}
}

// A database whose CA the API cannot serve (503) must fail with a message that
// names the cause, not with psql's opaque certificate error.
func TestProvisionReportsAnUnavailableServerCA(t *testing.T) {
	opts, env, _ := options(t, "psql", []string{"dpg-d0000000000000000000ab"},
		func(context.Context, string) (string, error) {
			return "", fmt.Errorf("%w (Postgres \"dpg-d0000000000000000000ab\" server CA is not provisioned yet; retry shortly)", ErrCAUnavailable)
		})

	release, err := Provision(context.Background(), opts)
	if err == nil {
		t.Fatal("an unavailable server CA was not reported")
	}
	release()
	if _, set := env[RootCertEnv]; set {
		t.Fatal("PGSSLROOTCERT was set even though no CA was obtained")
	}
	for _, want := range []string{"server CA", "verify-full", "dpg-d0000000000000000000ab", "psql"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error message does not mention %q: %v", want, err)
		}
	}
	if !errors.Is(err, ErrCAUnavailable) {
		t.Errorf("error lost its cause: %v", err)
	}
}

// An internal-only database has no external verify-full endpoint and serves no
// CA; that path must be left exactly as upstream runs it.
func TestProvisionLeavesInternalOnlyDatabaseUntouched(t *testing.T) {
	opts, env, _ := options(t, "psql", []string{"dpg-d0000000000000000000ab"}, servesCA(""))

	if _, err := Provision(context.Background(), opts); err != nil {
		t.Fatalf("internal-only database was refused: %v", err)
	}
	if _, set := env[RootCertEnv]; set {
		t.Fatal("PGSSLROOTCERT was set for a database with no external endpoint")
	}
}

// Every other read failure belongs to the delegated command, which reports it
// in upstream's own words.
func TestProvisionStaysSilentOnOtherFetchFailures(t *testing.T) {
	opts, env, _ := options(t, "psql", []string{"dpg-d0000000000000000000ab"},
		func(context.Context, string) (string, error) {
			return "", errors.New("run `render login` to authenticate")
		})

	if _, err := Provision(context.Background(), opts); err != nil {
		t.Fatalf("Provision turned an upstream-owned failure into its own: %v", err)
	}
	if _, set := env[RootCertEnv]; set {
		t.Fatal("PGSSLROOTCERT was set without a CA")
	}
}

func TestProvisionSkipsInvocationsItDoesNotOwn(t *testing.T) {
	for name, opts := range map[string]struct {
		tool string
		args []string
	}{
		"another command":        {tool: "services", args: []string{"dpg-d0000000000000000000ab"}},
		"interactive picker":     {tool: "psql", args: []string{"-o", "text"}},
		"dash before a selector": {tool: "pgcli", args: []string{"--", "--csv"}},
	} {
		t.Run(name, func(t *testing.T) {
			options, env, _ := options(t, opts.tool, opts.args, refuses(t))
			if _, err := Provision(context.Background(), options); err != nil {
				t.Fatalf("Provision: %v", err)
			}
			if _, set := env[RootCertEnv]; set {
				t.Fatal("PGSSLROOTCERT was set for an invocation with no resolvable database")
			}
		})
	}
}

func TestDatabaseArg(t *testing.T) {
	arity := psqlArity()
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"bare selector", []string{"my-db"}, "my-db"},
		{"after a bool flag", []string{"--confirm", "my-db"}, "my-db"},
		{"after a valued long flag", []string{"--command", "SELECT 1", "my-db"}, "my-db"},
		{"after an inline value", []string{"--command=SELECT 1", "my-db"}, "my-db"},
		{"before passthrough args", []string{"my-db", "--", "--csv"}, "my-db"},
		{"no selector", []string{"-c", "SELECT 1"}, ""},
		{"passthrough only", []string{"--", "--csv"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := DatabaseArg(tc.args, arity); got != tc.want {
				t.Fatalf("DatabaseArg(%q) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}
