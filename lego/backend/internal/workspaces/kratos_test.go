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

package workspaces

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// kratos_test.go covers the KratosIdentities admin-API reader: trait parsing
// (email + the optional w4/m25 name trait), MFA derivation, and the
// honest-omit contract on misses.

func TestKratosIdentitiesLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/admin/identities/named":
			fmt.Fprint(w, `{"traits":{"email":"a@example.com","name":"Ada Lovelace"},"credentials":{"totp":{"type":"totp"}}}`)
		case "/admin/identities/legacy":
			// An identity minted before the name trait existed.
			fmt.Fprint(w, `{"traits":{"email":"b@example.com"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	k := NewKratosIdentities(srv.URL)

	named, ok := k.Lookup(context.Background(), "named")
	if !ok || named.Email != "a@example.com" || named.Name != "Ada Lovelace" || !named.MFAEnabled {
		t.Fatalf("named = %+v, %v", named, ok)
	}

	legacy, ok := k.Lookup(context.Background(), "legacy")
	if !ok || legacy.Email != "b@example.com" || legacy.Name != "" || legacy.MFAEnabled {
		t.Fatalf("legacy = %+v, %v", legacy, ok)
	}

	if attrs, ok := k.Lookup(context.Background(), "missing"); ok {
		t.Fatalf("missing identity: want ok=false, got %+v", attrs)
	}
}

// TestKratosIdentitiesMFADerivation pins mfaEnabled per credential shape
// (w4/020): Kratos mints a stub webauthn entry (config.user_handle only, no
// registered keys) at password registration, so webauthn counts only when
// config.credentials is non-empty; totp stays presence-based.
func TestKratosIdentitiesMFADerivation(t *testing.T) {
	shapes := map[string]struct {
		credentials string
		want        bool
	}{
		"password-only":       {`{}`, false},
		"webauthn-stub":       {`{"webauthn":{"type":"webauthn","config":{"user_handle":"dXNlcg=="}}}`, false},
		"webauthn-empty-list": {`{"webauthn":{"type":"webauthn","config":{"credentials":[]}}}`, false},
		"webauthn-enrolled":   {`{"webauthn":{"type":"webauthn","config":{"credentials":[{"id":"a2V5","display_name":"key"}]}}}`, true},
		"totp":                {`{"totp":{"type":"totp"}}`, true},
		"totp-plus-stub":      {`{"totp":{"type":"totp"},"webauthn":{"type":"webauthn","config":{"user_handle":"dXNlcg=="}}}`, true},
		"totp-plus-enrolled":  {`{"totp":{"type":"totp"},"webauthn":{"type":"webauthn","config":{"credentials":[{"id":"a2V5"}]}}}`, true},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only webauthn's config is needed; asking for TOTP's would return the
		// secret (w5/127), and the shapes below are Kratos' answer to this ask.
		if included := r.URL.Query()["include_credential"]; !slices.Equal(included, []string{"webauthn"}) {
			t.Errorf("%s includes %v, want only webauthn", r.URL, included)
		}
		shape, ok := shapes[r.URL.Path[len("/admin/identities/"):]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `{"traits":{"email":"c@example.com"},"credentials":%s}`, shape.credentials)
	}))
	t.Cleanup(srv.Close)
	k := NewKratosIdentities(srv.URL)

	for name, shape := range shapes {
		attrs, ok := k.Lookup(context.Background(), name)
		if !ok || attrs.MFAEnabled != shape.want {
			t.Errorf("%s: mfaEnabled = %v (ok=%v), want %v", name, attrs.MFAEnabled, ok, shape.want)
		}
	}
}

// TestKratosIdentitiesEmailVerified pins EmailVerified to the verifiable
// address that matches the email trait: a verified OTHER address (a changed
// email whose old entry lingers) must not vouch for the current one.
func TestKratosIdentitiesEmailVerified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/admin/identities/verified":
			fmt.Fprint(w, `{"traits":{"email":"Ada@Example.com"},"verifiable_addresses":[{"value":"ada@example.com","verified":true}]}`)
		case "/admin/identities/pending":
			fmt.Fprint(w, `{"traits":{"email":"b@example.com"},"verifiable_addresses":[{"value":"b@example.com","verified":false}]}`)
		case "/admin/identities/other":
			fmt.Fprint(w, `{"traits":{"email":"new@example.com"},"verifiable_addresses":[{"value":"old@example.com","verified":true}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	k := NewKratosIdentities(srv.URL)

	for subject, want := range map[string]bool{"verified": true, "pending": false, "other": false} {
		attrs, ok := k.Lookup(context.Background(), subject)
		if !ok || attrs.EmailVerified != want {
			t.Errorf("%s: EmailVerified = %v (ok=%v), want %v", subject, attrs.EmailVerified, ok, want)
		}
	}
}

// fakeKratosList serves Kratos v26's GET /admin/identities?ids=… for every
// UUID except missing, records each request's ids, and refuses a whole batch
// over one id that is not a UUID, as Kratos does. failing names ids whose
// batch answers 500.
func fakeKratosList(t *testing.T, include []string, missing string, failing ...string) (*KratosIdentities, func() [][]string) {
	t.Helper()
	var mu sync.Mutex
	var batches [][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		mu.Lock()
		batches = append(batches, q["ids"])
		mu.Unlock()
		if r.URL.Path != "/admin/identities" || !slices.Equal(q["include_credential"], include) ||
			q.Get("page_size") != strconv.Itoa(len(q["ids"])) {
			t.Errorf("request %s, want one page of the batch read including credentials %v", r.URL, include)
		}
		for _, id := range q["ids"] {
			if uuid.Validate(id) != nil {
				http.Error(w, "invalid UUID value for parameter ids", http.StatusBadRequest)
				return
			}
			if slices.Contains(failing, id) {
				http.Error(w, "unavailable", http.StatusInternalServerError)
				return
			}
		}
		// Kratos lists credentials only when one is included (w5/127).
		credentials := ""
		if len(q["include_credential"]) > 0 {
			credentials = `,"credentials":{"totp":{"type":"totp"}}`
		}
		var found []string
		for _, id := range q["ids"] {
			if id != missing {
				found = append(found, fmt.Sprintf(`{"id":%q,"traits":{"email":"%s@example.com"}%s}`, id, id, credentials))
			}
		}
		fmt.Fprintf(w, "[%s]", strings.Join(found, ","))
	}))
	t.Cleanup(srv.Close)
	return NewKratosIdentities(srv.URL), func() [][]string {
		mu.Lock()
		defer mu.Unlock()
		return batches
	}
}

// TestListMembersResolvesItsIdentitiesInOneRequest (w5/116): listing members
// asked Kratos for each member's identity in turn, 20 serial requests for a
// 20-member workspace on every Team-page poll. One request now resolves them
// all. A member that is a named platform client, whose id would make Kratos
// refuse the whole batch, and a member Kratos does not know are left without
// identity fields.
func TestListMembersResolvesItsIdentitiesInOneRequest(t *testing.T) {
	unknown := uuid.NewString()
	k, batches := fakeKratosList(t, []string{"webauthn"}, unknown)
	st := newFakeStore()
	svc := &Service{Base: &core.Base{Authz: &fakeChecker{allow: true}}, Store: st, Identities: k}
	creator := uuid.NewString()
	w, err := svc.Create(ctxAs(creator), "acme", "hobby")
	if err != nil {
		t.Fatal(err)
	}
	for _, subject := range append([]string{"bex-bootstrap", unknown}, uuidsN(18)...) {
		st.members[w.ID] = append(st.members[w.ID], store.TenantMember{TenantID: w.ID, Subject: subject, Role: "developer"})
	}

	members, err := svc.ListMembers(ctxAs(creator), w.ID)
	if err != nil || len(members) != 21 {
		t.Fatalf("ListMembers: %d members, %v", len(members), err)
	}
	if got := batches(); len(got) != 1 || len(got[0]) != 20 {
		t.Fatalf("Kratos requests = %d (ids %v), want one carrying the 20 Kratos ids", len(got), got)
	}
	for _, m := range members {
		want := m.Subject + "@example.com"
		if m.Subject == "bex-bootstrap" || m.Subject == unknown {
			want = ""
		}
		if m.Email != want || m.MFAEnabled != (want != "") {
			t.Errorf("member %s = %q (MFA %v), want %q", m.Subject, m.Email, m.MFAEnabled, want)
		}
	}
}

// TestKratosIdentitiesLookupManyChunksAndFailsPerBatch (w5/116): ids go out
// kratosBatchSize at a time, and a failed batch leaves only its own subjects
// unresolved.
func TestKratosIdentitiesLookupManyChunksAndFailsPerBatch(t *testing.T) {
	subjects := uuidsN(2*kratosBatchSize + 50)
	poison := subjects[kratosBatchSize] // the second batch's first id
	k, batches := fakeKratosList(t, []string{"webauthn"}, "", poison)

	got := k.LookupMany(context.Background(), subjects)
	var sizes []int
	for _, batch := range batches() {
		sizes = append(sizes, len(batch))
	}
	slices.Sort(sizes)
	if !slices.Equal(sizes, []int{50, kratosBatchSize, kratosBatchSize}) {
		t.Fatalf("batch sizes = %v, want %d, %d, 50", sizes, kratosBatchSize, kratosBatchSize)
	}
	for i, subject := range subjects {
		_, resolved := got[subject]
		if failed := i >= kratosBatchSize && i < 2*kratosBatchSize; resolved == failed {
			t.Fatalf("subject %d resolved=%v, want only the failed batch's %d unresolved", i, resolved, kratosBatchSize)
		}
	}
}

// TestKratosIdentitiesLookupManyAnswersTheSubjectsAsked (w5/116): Kratos
// answers with its canonical, lowercase ids, and each result is keyed by the
// subject as asked, so a differently spelled subject still resolves and
// nothing unasked comes back.
func TestKratosIdentitiesLookupManyAnswersTheSubjectsAsked(t *testing.T) {
	lower, upper := uuid.NewString(), strings.ToUpper(uuid.NewString())
	k, _ := fakeKratosList(t, []string{"webauthn"}, "")
	got := k.LookupMany(context.Background(), []string{lower, upper, "bex-bootstrap"})
	if len(got) != 2 || got[lower].Email != lower+"@example.com" || got[upper].Email != strings.ToLower(upper)+"@example.com" {
		t.Fatalf("LookupMany = %v, want the two UUID subjects keyed as asked", got)
	}
}

// TestOwnerListsAskKratosForNoCredentials (w5/133): an owner list reports
// only each workspace's contact email, so its one batch read includes no
// credential. The member list, which reports MFA, still includes webauthn
// (TestListMembersResolvesItsIdentitiesInOneRequest).
func TestOwnerListsAskKratosForNoCredentials(t *testing.T) {
	k, batches := fakeKratosList(t, nil, "")
	svc := &Service{Base: &core.Base{Authz: &fakeChecker{allow: true}}, Store: newFakeStore(), Identities: k}
	owner := uuid.NewString()
	if _, err := svc.Create(ctxAs(owner), "acme", "pro"); err != nil {
		t.Fatal(err)
	}
	owners, err := svc.ListOwners(ctxAs(owner), OwnerFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(owners) != 1 || owners[0].Email != owner+"@example.com" || len(batches()) != 1 {
		t.Fatalf("owners = %+v after %d reads, want acme's owner email from one read", owners, len(batches()))
	}
}

// TestKratosIdentitiesLookupEmailsAsksForNoCredentials (w5/133): owner lists
// and the invite check read only emails, yet their batch read included the
// webauthn credential, so Kratos loaded the batch's credentials on every list.
// The email read includes none.
func TestKratosIdentitiesLookupEmailsAsksForNoCredentials(t *testing.T) {
	subjects := uuidsN(3)
	k, batches := fakeKratosList(t, nil, subjects[2])
	got, err := k.LookupEmails(context.Background(), subjects)
	if err != nil || len(batches()) != 1 || len(got) != 2 || got[subjects[0]] != subjects[0]+"@example.com" || got[subjects[1]] != subjects[1]+"@example.com" {
		t.Fatalf("LookupEmails = %v, want the two known subjects' emails from one read", got)
	}
}

func uuidsN(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = uuid.NewString()
	}
	return out
}
