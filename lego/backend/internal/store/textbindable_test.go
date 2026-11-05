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
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// w4/482 and w4/483: values PostgreSQL text cannot hold are recognised before
// binding, and the data exception that slips through is a caller error.
func TestTextBindableAndInvalidByteSequence(t *testing.T) {
	for in, want := range map[string]bool{"prj-abc": true, "téa": true, "\x00": false, "a\x00b": false, "\xc0\x80": false, "\xed\xa0\x80": false, "\xff\xfe": false} {
		if got := textBindable(in); got != want {
			t.Errorf("textBindable(%q) = %v, want %v", in, got, want)
		}
	}
	err := classify("project", &pgconn.PgError{Code: "22021"})
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("22021 = %v, want ErrInvalid", err)
	}
	if got := classify("project", errors.New("connection refused")); got.Error() != "connection refused" {
		t.Errorf("unrelated error rewritten: %v", got)
	}
}

// w4/504: the workspace-scoped registry-credential routes answer a malformed id
// as not-found before any query, exactly like GetRegistryCredentialByID. A nil
// pool would panic if a guard let the id through to the database.
func TestRegistryCredentialRoutesTreatMalformedIDsAsAbsent(t *testing.T) {
	s := &PGStore{}
	ctx := t.Context()
	for _, id := range []string{"\xff\xfe", "\xc0\x80", "a\x00b"} {
		if _, err := s.GetRegistryCredential(ctx, "tea-a", id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Get(%q) = %v, want ErrNotFound", id, err)
		}
		if _, err := s.UpdateRegistryCredential(ctx, "tea-a", id, "n", "u", nil); !errors.Is(err, ErrNotFound) {
			t.Errorf("Update(%q) = %v, want ErrNotFound", id, err)
		}
		if err := s.TouchRegistryCredential(ctx, "tea-a", id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Touch(%q) = %v, want ErrNotFound", id, err)
		}
		if err := s.DeleteRegistryCredential(ctx, "tea-a", id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Delete(%q) = %v, want ErrNotFound", id, err)
		}
	}
}
