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
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestSSHSessionAuditCannotCarryTerminalContent(t *testing.T) {
	wantFields := map[string]bool{
		"ID": true, "Subject": true, "WorkspaceID": true, "ServiceID": true,
		"InstanceID": true, "RemoteAddress": true, "StartedAt": true,
	}
	typeOfAudit := reflect.TypeOf(SSHSessionAudit{})
	if typeOfAudit.NumField() != len(wantFields) {
		t.Fatalf("SSHSessionAudit fields = %d, want exactly the content-free metadata set", typeOfAudit.NumField())
	}
	for i := 0; i < typeOfAudit.NumField(); i++ {
		field := typeOfAudit.Field(i).Name
		if !wantFields[field] {
			t.Fatalf("SSHSessionAudit gained unreviewed field %q", field)
		}
	}

	migration, err := os.ReadFile("migrations/0030_ssh_sessions.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := strings.ToLower(string(migration))
	for _, forbidden := range []string{"command", "argv", "environment", "stdin", "stdout", "stderr", "terminal"} {
		if strings.Contains(schema, forbidden) {
			t.Fatalf("ssh_sessions schema must not persist %q", forbidden)
		}
	}
}
