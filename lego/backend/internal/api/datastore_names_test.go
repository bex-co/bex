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

package api

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
)

// TestDatastoreNamesPassOneRule (w5/m118): Postgres and Key Value names pass
// one check on create and on rename (core.CheckDatastoreName): a
// free-form label, as on Render (w5/m153), including one shaped like a
// resource ID (w5/m118 reversed w8/049). One table over both kinds replaces a
// copy of this test per kind.
func TestDatastoreNamesPassOneRule(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})
	f := newParityFixture(&fakeChecker{allow: true}, false)
	// The fixture seeds one free database and store; move them to paid so
	// the one-free slot (w4/m241) is open for the creates under test.
	flipSeededDatastorePlans(t, f)
	kinds := map[string]map[string]func(name string) error{
		"Postgres": {
			"create": func(name string) error {
				_, err := f.pg.CreatePostgres(ctx, postgres.CreatePostgresRequest{Name: name, Plan: "free", DryRun: true})
				return err
			},
			"rename": func(name string) error {
				_, err := f.pg.UpdatePostgresDryRun(ctx, "dpg-parity", postgres.PostgresPatch{Name: &name})
				return err
			},
		},
		"Key Value": {
			"create": func(name string) error {
				_, err := f.kv.CreateKeyValue(ctx, keyvalue.CreateKeyValueRequest{Name: name, Plan: "free", DryRun: true})
				return err
			},
			"rename": func(name string) error {
				_, err := f.kv.UpdateKeyValueDryRun(ctx, "red-parity", keyvalue.KeyValuePatch{Name: &name})
				return err
			},
		},
	}
	for kind, verbs := range kinds {
		for verb, call := range verbs {
			for name, code := range map[string]string{
				"billing": "", "srv-web": "", "dpg-db1e4vnhb1uc73ebifi": "",
				"srv-db1e4vnhb1uc73ebifi0": "",
				"dpg-zzzzzzzzzzzzzzzzzzzz": "",
				"red-aaaaaaaaaaaaaaaaaaaa": "",
				"Billing":                  "",
				"cache_1":                  "",
				"Prod DB":                  "",
				" padded":                  "RESOURCE_NAME_INVALID",
			} {
				err := call(name)
				var coded *core.CodedError
				switch {
				case code == "" && err != nil:
					t.Errorf("%s %s %q = %v, want accepted", kind, verb, name, err)
				case code != "" && (!errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) || coded.Code != code || coded.Params["field"] != "name"):
					t.Errorf("%s %s %q = %v, want %s for field name", kind, verb, name, err, code)
				}
			}
		}
	}
}
