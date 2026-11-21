/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
)

// logicalReplicationCommand is Render support's "enable logical replication"
// (render.com/docs/postgresql-logical-replication; w34/m541), run inside the
// bex-api pod on a workspace's request:
//
//	kubectl -n bex-system exec deploy/bex-api -- /api postgres-logical-replication enable --database dpg-… --role <credential> --schema public [--schema …] [--all-tables]
//	kubectl -n bex-system exec deploy/bex-api -- /api postgres-logical-replication disable --database dpg-…
//
// It is a thin client of the running server's :8091 verb, like
// platform-rebuild, so the control-plane token never leaves the pod. It
// prints the verb's JSON answer; the exit code is 0 only once the database
// itself reports the change (docs/runbooks/postgres-logical-replication.md).
const logicalReplicationCommand = "postgres-logical-replication"

const logicalReplicationUsage = "usage: " + logicalReplicationCommand + " enable --database <dpg-id> --role <credential> --schema <name> [--schema <name>…] [--all-tables]\n" +
	"       " + logicalReplicationCommand + " disable --database <dpg-id>"

// schemaFlags collects repeated --schema values.
type schemaFlags []string

func (f *schemaFlags) String() string     { return strings.Join(*f, ",") }
func (f *schemaFlags) Set(v string) error { *f = append(*f, v); return nil }

func parseLogicalReplicationArgs(args []string) (postgres.LogicalReplicationRequest, error) {
	usage := errors.New(logicalReplicationUsage)
	if len(args) == 0 || (args[0] != "enable" && args[0] != "disable") {
		return postgres.LogicalReplicationRequest{}, usage
	}
	enable := args[0] == "enable"
	fs := flag.NewFlagSet(logicalReplicationCommand, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	database := fs.String("database", "", "the database's dpg- id")
	var role *string
	var schemas schemaFlags
	var allTables *bool
	if enable {
		role = fs.String("role", "", "the credential subscribers connect as")
		fs.Var(&schemas, "schema", "a schema subscribers read (repeatable)")
		allTables = fs.Bool("all-tables", false, "create the FOR ALL TABLES publication")
	}
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() > 0 || !strings.HasPrefix(*database, ids.Postgres.Prefix()+"-") {
		return postgres.LogicalReplicationRequest{}, usage
	}
	req := postgres.LogicalReplicationRequest{Database: *database, Enable: enable}
	if enable {
		if *role == "" || len(schemas) == 0 {
			return postgres.LogicalReplicationRequest{}, usage
		}
		req.PublisherRole, req.Schemas, req.AllTables = *role, schemas, *allTables
	}
	return req, nil
}

// runLogicalReplication posts the request to the local :8091 verb, prints
// the JSON answer, and returns the exit code: 0 converged, 3 written but not
// yet reported by the database, 1 refused or failed, 2 a usage error.
func runLogicalReplication(ctx context.Context, cfg *Config, args []string, out, errOut io.Writer) int {
	req, err := parseLogicalReplicationArgs(args)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	answer, code := postControlPlaneVerb(ctx, cfg, logicalReplicationCommand, postgres.LogicalReplicationPath, req, 5*time.Minute, out, errOut)
	if code != 0 {
		return code
	}
	var res postgres.LogicalReplicationResult
	if err := json.Unmarshal(answer, &res); err != nil {
		fmt.Fprintf(errOut, "%s: the answer does not parse: %v\n", logicalReplicationCommand, err)
		return 1
	}
	if !res.Converged {
		fmt.Fprintf(errOut, "%s: written, but the database has not reported the change yet; check again with the same command\n", logicalReplicationCommand)
		return 3
	}
	return 0
}
