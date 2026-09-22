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
package store

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Both censuses inspect migrated PostgreSQL, including schema changes made in
// later migrations. A transaction lets mutation proofs roll back their DDL.
type schemaQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func schemaColumns(ctx context.Context, db schemaQuerier, pattern string) ([]string, error) {
	rows, err := db.Query(ctx, `
		SELECT c.table_name || '.' || c.column_name
		FROM information_schema.columns c
		JOIN information_schema.tables t USING (table_catalog, table_schema, table_name)
		WHERE c.table_schema = 'public' AND t.table_type = 'BASE TABLE'
		  AND c.column_name ~ $1
		ORDER BY c.table_name, c.column_name`, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func checkSchemaDispositions(columns []string, inventory map[string]string) error {
	live := make(map[string]bool, len(columns))
	var problems []string
	for _, column := range columns {
		live[column] = true
		if _, ok := inventory[column]; !ok {
			problems = append(problems, "undeclared "+column)
		}
	}
	for column, disposition := range inventory {
		if !live[column] {
			problems = append(problems, "stale declaration "+column)
		}
		kind, reason, _ := strings.Cut(disposition, ":")
		switch kind {
		case "delete", "anonymize", "cascade", "purger", "retain", "retention":
		default:
			problems = append(problems, "invalid disposition for "+column)
		}
		if strings.TrimSpace(reason) == "" {
			problems = append(problems, "missing disposition reason for "+column)
		}
	}
	if len(problems) > 0 {
		slices.Sort(problems)
		return fmt.Errorf("schema disposition inventory: %s", strings.Join(problems, "; "))
	}
	return nil
}
