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

package github

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// staleAllowChecker models the round-15 #3 window: Check still answers a warm
// positive while CheckFresh already says the initiator is no longer an admin.
type staleAllowChecker struct{}

func (staleAllowChecker) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

func (staleAllowChecker) CheckFresh(context.Context, string, string, string) (bool, error) {
	return false, nil
}

type allowFreshChecker struct{}

func (allowFreshChecker) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

func (allowFreshChecker) CheckFresh(context.Context, string, string, string) (bool, error) {
	return true, nil
}

// A demoted initiator who still holds a consumed-once nonce and GitHub admin
// proof must not bind the installation; the store stays empty.
func TestConnectCallbackFailsClosedOnFreshRevocation(t *testing.T) {
	svc := connectService(t)
	svc.Authz = staleAllowChecker{}
	nonce := seedConnectTxn(t, svc, attackerWorkspce, attackerSubject)

	if _, err := svc.connectFromCallback(context.Background(), nonce, attackerSubject, 42, "oauth-code"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("stale-positive callback = %v, want ErrForbidden", err)
	}
	if len(svc.Store.(*fakeStore).conns) != 0 {
		t.Fatal("fresh deny must not upsert git_connections")
	}
}

// A current admin still completes the callback when Authz is wired.
func TestConnectCallbackSucceedsWithCurrentAdmin(t *testing.T) {
	svc := connectService(t)
	svc.Authz = allowFreshChecker{}
	nonce := seedConnectTxn(t, svc, attackerWorkspce, attackerSubject)

	conn, err := svc.connectFromCallback(context.Background(), nonce, attackerSubject, 42, "oauth-code")
	if err != nil {
		t.Fatalf("current-admin callback: %v", err)
	}
	if !conn.Connected || conn.InstallationID != 42 {
		t.Fatalf("connection = %+v", conn)
	}
}
