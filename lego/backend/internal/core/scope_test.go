/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

    10|Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package core

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

func TestIdentityFromNilContext(t *testing.T) {
	if _, ok := IdentityFrom(nil); ok {
		t.Fatal("nil context must report no identity")
	}
	if err := checkCapability(nil, RelCanView); err != nil {
		t.Fatalf("no identity is scope-exempt, got %v", err)
	}
}

func TestRelCanRelationsAreFullyMapped(t *testing.T) {
	mapped := map[string]string{}
	for _, rel := range RelCanRelations() {
		cap, ok := RequiredCapability(rel)
		if !ok || cap == "" {
			t.Errorf("relation %q is unmapped", rel)
			continue
		}
		mapped[rel] = cap
	}
	if len(mapped) != len(relationCapability) {
		t.Errorf("RelCanRelations (%d) drifted from relationCapability (%d)", len(mapped), len(relationCapability))
	}
	if _, ok := RequiredCapability("can_invented"); ok {
		t.Error("unknown relation must fail closed")
	}
}

func TestNormalizeOAuthGrantCanonicalizesAndBounds(t *testing.T) {
	g, err := NormalizeOAuthGrant("openid bex.write bex.read bex.write email", []string{"https://other", "https://api.bex.co/mcp"}, "client-1", "https://api.bex.co/mcp")
	if err != nil {
		t.Fatal(err)
	}
	if g.Scopes != "bex.read bex.write" {
		t.Errorf("scopes = %q, want sorted unique closed vocab", g.Scopes)
	}
	if g.AcceptedAudience != "https://api.bex.co/mcp" {
		t.Errorf("accepted audience = %q, want the resource this API accepted", g.AcceptedAudience)
	}
	if !g.HasGranular() {
		t.Error("granular grant must report HasGranular")
	}

	near, err := NormalizeOAuthGrant("bex.read-only bex.api-readonly bex", nil, "c", "")
	if err != nil {
		t.Fatal(err)
	}
	if near.Scopes != "" {
		t.Errorf("near-matches retained %q", near.Scopes)
	}

	if _, err := NormalizeOAuthGrant(strings.Repeat("x", MaxOAuthScopeLen+1), nil, "c", ""); err == nil {
		t.Error("oversized scope must fail closed")
	}
	if _, err := NormalizeOAuthGrant("bex.read", nil, strings.Repeat("c", MaxOAuthClientIDLen+1), ""); err == nil {
		t.Error("oversized client id must fail closed")
	}
}

func TestHasGranularCapabilityExactMatchOnly(t *testing.T) {
	if HasGranularCapability("bex.read-only") || ContainsScope("bex.read-only", ScopeRead) {
		t.Error("prefix near-match must not count")
	}
	if HasGranularCapability("xbex.read") || ContainsScope("not bex.readx", ScopeRead) {
		t.Error("substring near-match must not count")
	}
	if !ContainsScope("bex.read bex.write", ScopeRead) {
		t.Error("exact member must count")
	}
}

type countingAllowChecker struct{ n atomic.Int32 }

func (c *countingAllowChecker) Check(context.Context, string, string, string) (bool, error) {
	c.n.Add(1)
	return true, nil
}

func scopedHuman(scopes string, platform bool) Identity {
	return Identity{
		Subject:          "user-a",
		Method:           "oauth2",
		ClientID:         "dcr-client",
		Human:            true,
		CanonicalScopes:  scopes,
		AcceptedAudience: "https://api.bex.co/mcp",
		PlatformClient:   platform,
	}
}

func TestCapabilityMatrixAtSharedSeam(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	session := Identity{Subject: "user-a", Method: "session"}
	cases := []struct {
		name      string
		id        Identity
		relation  string
		wantAllow bool
		wantCap   string
	}{
		{name: "read permits view", id: scopedHuman(ScopeRead, false), relation: RelCanView, wantAllow: true},
		{name: "read permits logs", id: scopedHuman(ScopeRead, false), relation: RelCanViewLogs, wantAllow: true},
		{name: "read cannot operate", id: scopedHuman(ScopeRead, false), relation: RelCanOperate, wantCap: ScopeWrite},
		{name: "read cannot create", id: scopedHuman(ScopeRead, false), relation: RelCanCreate, wantCap: ScopeWrite},
		{name: "read cannot view sensitive", id: scopedHuman(ScopeRead, false), relation: RelCanViewSensitive, wantCap: ScopeSensitive},
		{name: "write permits operate", id: scopedHuman(ScopeWrite, false), relation: RelCanOperate, wantAllow: true},
		{name: "write permits create", id: scopedHuman(ScopeWrite, false), relation: RelCanCreate, wantAllow: true},
		{name: "write permits manage", id: scopedHuman(ScopeWrite, false), relation: RelCanManage, wantAllow: true},
		{name: "write permits billing", id: scopedHuman(ScopeWrite, false), relation: RelCanManageBilling, wantAllow: true},
		{name: "write permits keys", id: scopedHuman(ScopeWrite, false), relation: RelCanManageKeys, wantAllow: true},
		{name: "write permits ssh keys", id: scopedHuman(ScopeWrite, false), relation: RelCanManageSSHKeys, wantAllow: true},
		{name: "write cannot view sensitive", id: scopedHuman(ScopeWrite, false), relation: RelCanViewSensitive, wantCap: ScopeSensitive},
		{name: "sensitive permits sensitive", id: scopedHuman(ScopeSensitive, false), relation: RelCanViewSensitive, wantAllow: true},
		{name: "sensitive cannot view", id: scopedHuman(ScopeSensitive, false), relation: RelCanView, wantCap: ScopeRead},
		{name: "sensitive cannot write", id: scopedHuman(ScopeSensitive, false), relation: RelCanOperate, wantCap: ScopeWrite},
		{name: "legacy third-party bex.api cannot read", id: scopedHuman(ScopeAPICompatibility, false), relation: RelCanView, wantCap: ScopeRead},
		{name: "near-match cannot read", id: scopedHuman("bex.read-only", false), relation: RelCanView, wantCap: ScopeRead},
		{name: "unknown relation fails closed", id: scopedHuman(ScopeRead+" "+ScopeWrite+" "+ScopeSensitive, false), relation: "can_invented", wantCap: ""},
		{name: "platform legacy exempt", id: scopedHuman(ScopeAPICompatibility, true), relation: RelCanManage, wantAllow: true},
		{name: "platform granular is narrowed", id: scopedHuman(ScopeRead, true), relation: RelCanOperate, wantCap: ScopeWrite},
		{name: "session exempt", id: session, relation: RelCanManage, wantAllow: true},
		{name: "machine exempt", id: Identity{Subject: "user-a", Method: "oauth2", ClientID: "key-1"}, relation: RelCanManage, wantAllow: true},
		{name: "oauth2 without Human is machine-class", id: Identity{Subject: "user-a", Method: "oauth2"}, relation: RelCanCreate, wantAllow: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chk := &countingAllowChecker{}
			sink := &fakeAuditSink{}
			b := &Base{Authz: chk, Workspace: ws, Audit: sink}
			ctx := WithIdentity(context.Background(), tc.id)
			err := b.Authorize(ctx, tc.relation)
			if tc.wantAllow {
				if err != nil {
					t.Fatalf("Authorize = %v, want nil", err)
				}
				if chk.n.Load() == 0 {
					t.Error("allowed path must still consult OpenFGA")
				}
				if !b.Can(ctx, tc.relation) {
					t.Error("Can must match Authorize")
				}
				return
			}
			var coded *CodedError
			if !errors.As(err, &coded) || coded.Code != InsufficientScopeCode {
				t.Fatalf("Authorize = %v, want INSUFFICIENT_SCOPE", err)
			}
			if tc.wantCap == "" {
				if coded.Params["required"] != nil {
					t.Errorf("unknown relation params = %v, want no required", coded.Params)
				}
			} else if coded.Params["required"] != tc.wantCap {
				t.Errorf("required = %v, want %s", coded.Params["required"], tc.wantCap)
			}
			if chk.n.Load() != 0 {
				t.Error("capability denial must not call OpenFGA")
			}
			if writeRelations[tc.relation] || tc.relation == RelCanView || tc.relation == RelCanViewLogs || tc.relation == RelCanViewSensitive {
				if sink.len() != 1 {
					t.Errorf("denied audit count = %d, want 1", sink.len())
				} else if sink.events[0].Relation != tc.relation || sink.events[0].Outcome != AuditDenied {
					t.Errorf("audit = %+v", sink.events[0])
				}
			}
			if b.Can(ctx, tc.relation) {
				t.Error("Can must apply the same matrix")
			}
		})
	}
}

func TestCapabilityDenialIsSingleAuditAndCanOmitsSensitive(t *testing.T) {
	chk := &countingAllowChecker{}
	sink := &fakeAuditSink{}
	b := &Base{Authz: chk, Workspace: fakeWorkspace{"user-a": "tea-a"}, Audit: sink}
	ctx := WithIdentity(context.Background(), scopedHuman(ScopeRead, false))
	if err := b.Authorize(ctx, RelCanOperate); err == nil {
		t.Fatal("write must fail")
	}
	if sink.len() != 1 {
		t.Fatalf("denied write audits = %d, want 1", sink.len())
	}
	if b.Can(ctx, RelCanViewSensitive) {
		t.Error("Can(RelCanViewSensitive) must omit under bex.read")
	}
	if chk.n.Load() != 0 {
		t.Error("Can/Authorize scope denials must not call OpenFGA")
	}
}

func TestRequireOpClass(t *testing.T) {
	read := scopedHuman(ScopeRead, false)
	write := scopedHuman(ScopeWrite, false)
	sensitive := scopedHuman(ScopeSensitive, false)
	session := Identity{Subject: "u", Method: "session", Human: true}
	key := Identity{Subject: "key-1", Method: "oauth2", ClientID: "key-1", Human: false}
	platform := Identity{
		Subject: "u", Method: "oauth2", ClientID: "bex-mobile", Human: true,
		PlatformClient: true, CanonicalScopes: ScopeAPICompatibility,
	}

	if err := read.RequireOpClass(OpClassRead); err != nil {
		t.Fatalf("read class with bex.read: %v", err)
	}
	if err := read.RequireOpClass(OpClassWrite); err == nil {
		t.Fatal("read token must not satisfy write class")
	}
	if err := read.RequireOpClass(OpClassMint); err == nil {
		t.Fatal("read token must not satisfy mint class")
	}
	if err := write.RequireOpClass(OpClassMint); err != nil {
		t.Fatalf("mint class is write at dispatch: %v", err)
	}
	if err := write.RequireOpClass(OpClassSensitive); err == nil {
		t.Fatal("write token must not satisfy sensitive class")
	}
	if err := sensitive.RequireOpClass(OpClassSensitive); err != nil {
		t.Fatalf("sensitive class: %v", err)
	}
	if err := session.RequireOpClass(OpClassMint); err != nil {
		t.Fatalf("session is exempt: %v", err)
	}
	if err := key.RequireOpClass(OpClassWrite); err != nil {
		t.Fatalf("api key is exempt: %v", err)
	}
	if err := platform.RequireOpClass(OpClassSensitive); err != nil {
		t.Fatalf("platform client without granular grant is exempt: %v", err)
	}
	if err := read.RequireOpClass("invented"); err == nil {
		t.Fatal("unknown class must fail closed")
	}
}

func TestInsufficientScopeCrossSurfaceProjection(t *testing.T) {
	err := NewInsufficientScopeError(ScopeWrite)
	if !errors.Is(err, ErrForbidden) {
		t.Error("must wrap ErrForbidden (403)")
	}
	ext := err.Extensions()
	if ext["code"] != InsufficientScopeCode || ext["required"] != ScopeWrite {
		t.Errorf("graphql extensions = %v", ext)
	}
	mcp := MCPError(err).Error()
	if !strings.HasPrefix(mcp, InsufficientScopeCode+":") {
		t.Errorf("mcp = %q", mcp)
	}
}

func TestIdentityZeroComparabilityAndEmptyOAuthFields(t *testing.T) {
	if (Identity{}) != (Identity{}) {
		t.Fatal("empty Identity must remain comparable")
	}
	session := Identity{Subject: "u", Method: "session", Human: true}
	if session.CanonicalScopes != "" || session.AcceptedAudience != "" || session.PlatformClient {
		t.Errorf("session acquired oauth fields: %+v", session)
	}
	machine := Identity{Subject: "k", Method: "oauth2", ClientID: "k"}
	if !machine.CapabilityExempt() {
		t.Error("machine token must be scope-exempt")
	}
	ev := AuditEvent{}
	machine.AttachOAuthProvenance(&ev)
	if ev.OAuthClientID != "" || len(ev.OAuthScopes) != 0 {
		t.Errorf("machine provenance leaked: %+v", ev)
	}
}

func TestOpenFGAStillRequiredAfterPassingScope(t *testing.T) {
	deny := &fakeDenyChecker{}
	b := &Base{Authz: deny, Workspace: fakeWorkspace{"user-a": "tea-a"}}
	ctx := WithIdentity(context.Background(), scopedHuman(ScopeWrite, false))
	err := b.Authorize(ctx, RelCanOperate)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Authorize = %v, want OpenFGA ErrForbidden", err)
	}
	var coded *CodedError
	if errors.As(err, &coded) {
		t.Error("OpenFGA denial must not be rewritten as insufficient_scope")
	}
}
