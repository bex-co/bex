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
	"crypto/sha256"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// htpasswdKey is the data key of the Zot htpasswd Secret, matching the file
// the chart mounts at zotHTPasswdPath.
const htpasswdKey = "htpasswd"

// verifyDigest keys a memoized bcrypt comparison by its exact inputs. Any
// change to either the stored hash or the password yields a different digest,
// so a memo hit is only ever reached for a pair the real compare already
// accepted — the memo can delay no drift.
func verifyDigest(hash []byte, password string) [sha256.Size]byte {
	// The NUL separator keeps ("ab","c") from colliding with ("a","bc").
	return sha256.Sum256(append(append(append([]byte{}, hash...), 0), password...))
}

// hashVerified reports whether this exact (hash, password) pair has already
// been verified for username.
//
// bcrypt.CompareHashAndPassword is pure but deliberately expensive, and the App
// reconciler re-runs it for every App on every pass — it is level-triggered
// with no ObservedGeneration early-return, and a Running App requeues every
// ~30s. At bcrypt.DefaultCost that is ~60-100ms of CPU per App per pass,
// serialized through BEX_APP_RECONCILE_WORKERS, so at a few hundred Apps it
// caps reconcile throughput on work whose answer never changed.
func (c *Creds) hashVerified(username string, hash []byte, password string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	got, ok := c.verified[username]
	return ok && got == verifyDigest(hash, password)
}

// recordVerified memoizes a successful comparison. Only the digest is retained.
func (c *Creds) recordVerified(username string, hash []byte, password string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.verified == nil {
		c.verified = map[string][sha256.Size]byte{}
	}
	c.verified[username] = verifyDigest(hash, password)
}

// forgetVerified drops username's memo, so the next ensure re-derives it. Every
// path that rewrites or removes the entry calls this; a stale memo would
// otherwise have to be invalidated by the hash change alone, which holds today
// but is not a property those callers should have to preserve.
func (c *Creds) forgetVerified(username string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.verified, username)
}

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
