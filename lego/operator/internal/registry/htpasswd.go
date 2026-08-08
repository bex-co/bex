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

package registry

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// htpasswdKey is the data key of the Zot htpasswd Secret, matching the file
// the chart mounts at zotHTPasswdPath.
const htpasswdKey = "htpasswd"

// htpasswdUserHash returns the bcrypt hash stored for username.
func htpasswdUserHash(htpasswd []byte, username string) (hash []byte, found bool) {
	prefix := username + ":"
	for line := range strings.SplitSeq(string(htpasswd), "\n") {
		if h, ok := strings.CutPrefix(line, prefix); ok {
			return []byte(h), true
		}
	}
	return nil, false
}

// htpasswdWithout returns htpasswd's non-empty lines minus username's entry.
func htpasswdWithout(htpasswd []byte, username string) []string {
	prefix := username + ":"
	var lines []string
	for line := range strings.SplitSeq(strings.TrimRight(string(htpasswd), "\n"), "\n") {
		if line == "" || strings.HasPrefix(line, prefix) {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

// joinHTPasswd renders htpasswd lines back to file content. A drained file is
// nil rather than a bare newline, so a fully revoked registry holds no bytes.
func joinHTPasswd(lines []string) []byte {
	if len(lines) == 0 {
		return nil
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

// setHTPasswdLine gives username a freshly hashed password, replacing any
// entry it already has.
func setHTPasswdLine(htpasswd []byte, username, password string) ([]byte, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return joinHTPasswd(append(htpasswdWithout(htpasswd, username), username+":"+string(hash))), nil
}

// removeHTPasswdLine drops username's entry.
func removeHTPasswdLine(htpasswd []byte, username string) []byte {
	return joinHTPasswd(htpasswdWithout(htpasswd, username))
}
