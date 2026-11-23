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
	// Transport (w6/m564) is a closed set of three names (CHECK in 0339): how
	// the connection arrived, never what it carried.
	wantFields := map[string]bool{
		"ID": true, "Subject": true, "WorkspaceID": true, "ServiceID": true,
		"InstanceID": true, "RemoteAddress": true, "StartedAt": true, "Transport": true,
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

	for _, path := range []string{"migrations/0030_ssh_sessions.up.sql", "migrations/0339_ssh_sessions_transport.up.sql"} {
		migration, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		schema := strings.ToLower(string(migration))
		for _, forbidden := range []string{"command", "argv", "environment", "stdin", "stdout", "stderr", "terminal"} {
			if strings.Contains(schema, forbidden) {
				t.Fatalf("%s: ssh_sessions schema must not persist %q", path, forbidden)
			}
		}
	}
}
