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
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestPushTokenDigestIsStableAndDomainSeparated(t *testing.T) {
	a := pushTokenDigest("expo", "secret")
	if len(a) != 64 || a != pushTokenDigest("expo", "secret") {
		t.Fatalf("digest = %q", a)
	}
	if a == pushTokenDigest("other", "secret") || a == pushTokenDigest("expo", "other") {
		t.Fatal("push token digest is not provider/token domain-separated")
	}
}

func TestPushSubscriptionDatabaseErrorsNeverLeakFailingToken(t *testing.T) {
	secret := "ExponentPushToken[do-not-log-me]"
	err := classifyPushSubscriptionError(&pgconn.PgError{
		Code: "23514", ConstraintName: "device_push_subscriptions_token_check",
		Detail: "Failing row contains (tea-a, alice, phone, expo, ios, " + secret + ")",
	})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("classified error = %v", err)
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "Failing row") {
		t.Fatalf("classified error leaked database detail: %v", err)
	}
}
