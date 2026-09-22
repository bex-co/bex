// Package pgtrust provisions the TLS trust a bex Postgres session needs before
// the launcher hands `psql`/`pgcli` to the upstream Render CLI.
//
// bex-api pins `sslmode=verify-full` on every external Postgres connection
// string (lego/backend/internal/postgres/service.go, w4/m95), and the issuing
// CA is the CNPG cluster's private one — no stock client holds it. The pinned
// upstream CLI passes that connection string to a real `psql` binary verbatim
// (pkg/tui/views/psql.go) and has no notion of a bex CA field, so a stock
// install fails with `root certificate file "~/.postgresql/root.crt" does not
// exist` before any SQL runs.
//
// The launcher closes that gap the same way it owns the root `--version` path:
// it intercepts the invocation before delegating, reads the database's
// `serverCaCertificate` from the authenticated connection-info endpoint, writes
// it to a private temporary file, and points PGSSLROOTCERT at it. The upstream
// CLI execs `psql` with the launcher's environment, so the child inherits the
// trust root; nothing about the request/response contract or the connection
// string changes, and TLS is never downgraded.
//
// An explicit user choice always wins: a set PGSSLROOTCERT or an existing
// ~/.postgresql/root.crt (libpq's implicit per-user trust store) is left alone.
package pgtrust

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
)

// RootCertEnv is libpq's trusted-root override, honored by psql and by pgcli
// (which connects through libpq via psycopg).
const RootCertEnv = "PGSSLROOTCERT"

// userRootCert is libpq's implicit per-user trust store, relative to $HOME. Its
// presence means the user already curates their own trust; the launcher then
// provisions nothing so it cannot shadow that choice.
var userRootCert = filepath.Join(".postgresql", "root.crt")

// ErrCAUnavailable marks a connection-info read that answered "the server CA
// does not exist yet" (bex-api replies 503). It is the one fetch failure the
// user must be told about: the session provably cannot open, so a clear message
// beats the opaque psql failure that would follow.
var ErrCAUnavailable = errors.New("the database's TLS server CA is not available yet")

// IsProvisionedTool reports whether a subcommand opens a libpq session that the
// launcher provisions trust for.
func IsProvisionedTool(name string) bool {
	return name == "psql" || name == "pgcli"
}

// FlagArity reports, per flag token ("--command", "-c"), whether the token
// consumes the argument that follows it. DatabaseArg needs exactly this much of
// the command's flag definitions to tell a flag's value from a positional.
type FlagArity map[string]bool

// ArityOf collects the arity of every flag in sets. Nil sets are skipped so
// callers can pass a command's local, inherited, and root persistent flags
// without checking which exist.
func ArityOf(sets ...*pflag.FlagSet) FlagArity {
	arity := make(FlagArity)
	for _, set := range sets {
		if set == nil {
			continue
		}
		set.VisitAll(func(flag *pflag.Flag) {
			// Mirrors the upstream detector: a bool flag, or any flag with an
			// optional value, never consumes the next argument.
			takesValue := flag.Value.Type() != "bool" && flag.NoOptDefVal == ""
			arity["--"+flag.Name] = takesValue
			if flag.Shorthand != "" {
				arity["-"+flag.Shorthand] = takesValue
			}
		})
	}
	return arity
}

// DatabaseArg returns the database ID or name a `psql`/`pgcli` invocation
// targets, or "" when the invocation names none.
//
// args is the subcommand's own argv. The scan mirrors upstream's own reading of
// it: everything from `--` onward belongs to the child tool, so a leading `--`
// means the user is picking the database interactively (upstream clears the
// selector when ArgsLenAtDash is 0) and there is nothing to resolve here.
func DatabaseArg(args []string, arity FlagArity) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return ""
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			return arg
		}
		// `--flag=value` carries its value inline; a flag the command does not
		// define is assumed valueless, which at worst costs a lookup that fails
		// and leaves the invocation exactly as upstream would have run it.
		if strings.Contains(arg, "=") {
			continue
		}
		if arity[arg] {
			i++
		}
	}
	return ""
}

// Options carries everything Provision needs. Every process-level dependency is
// injected so the decision table is testable without a network, a home
// directory, or a real environment.
type Options struct {
	// Tool is the intercepted subcommand's name ("psql" or "pgcli").
	Tool string
	// Args is that subcommand's own argv, flags included.
	Args []string
	// Arity describes the flags Args may contain.
	Arity FlagArity
	// Fetch reads a database's PEM server CA. It returns "" when the database
	// has no external, verify-full endpoint (an internal-only database needs no
	// trust provisioning), and an ErrCAUnavailable-wrapped error when the CA is
	// missing on the server side.
	Fetch func(ctx context.Context, idOrName string) (string, error)

	Lookup func(string) (string, bool)
	Setenv func(string, string) error
	Home   func() (string, error)
	// Dir holds the temporary CA file; empty means the OS temp directory.
	Dir string
}

// Provision points PGSSLROOTCERT at the target database's server CA.
//
// The returned cleanup is never nil and must be called once the delegated
// command has finished. A non-nil error means the session cannot open and the
// user needs to know why; every other failure mode returns no error and no
// provisioning, leaving the invocation byte-identical to an unmodified upstream
// run.
func Provision(ctx context.Context, opts Options) (func(), error) {
	noop := func() {}
	if !IsProvisionedTool(opts.Tool) {
		return noop, nil
	}
	target := DatabaseArg(opts.Args, opts.Arity)
	if target == "" {
		// The database is picked from the interactive table after the launcher
		// has handed off, so there is nothing to resolve here.
		return noop, nil
	}
	if value, _ := opts.Lookup(RootCertEnv); strings.TrimSpace(value) != "" {
		return noop, nil
	}
	if home, err := opts.Home(); err == nil && home != "" {
		if _, err := os.Stat(filepath.Join(home, userRootCert)); err == nil {
			return noop, nil
		}
	}

	certificate, err := opts.Fetch(ctx, target)
	if err != nil {
		if errors.Is(err, ErrCAUnavailable) {
			return noop, fmt.Errorf(
				"cannot open a %s session to Postgres %q: %w.\n"+
					"A public bex Postgres pins sslmode=verify-full, so the session cannot open until "+
					"its server CA exists. Retry once the database reports available; if it persists, "+
					"the database's external endpoint is not provisioned",
				opts.Tool, target, err)
		}
		// Anything else — not logged in, no such database, an unreachable API —
		// is a failure the delegated command reports for itself, in upstream's
		// own words. Staying silent here keeps that message the only one.
		return noop, nil
	}
	if strings.TrimSpace(certificate) == "" {
		// An internal-only database serves no CA and no external string; its
		// path must keep working untouched.
		return noop, nil
	}

	path, err := writeCertificate(opts.Dir, certificate)
	if err != nil {
		return noop, nil
	}
	remove := func() { _ = os.Remove(path) }
	if err := opts.Setenv(RootCertEnv, path); err != nil {
		remove()
		return noop, nil
	}
	return remove, nil
}

// writeCertificate stores the PEM bundle in a private (0600) temporary file.
// The CA is public material, but the file is still the process's own: a
// world-readable path invites another local user to swap the trust root the
// child is about to verify against.
func writeCertificate(dir, certificate string) (string, error) {
	file, err := os.CreateTemp(dir, "bex-postgres-ca-*.crt")
	if err != nil {
		return "", err
	}
	if _, err := file.WriteString(certificate); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}
