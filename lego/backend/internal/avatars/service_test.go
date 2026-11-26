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

package avatars

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type allowChecker struct{}

func (allowChecker) Check(context.Context, string, string, string) (bool, error) { return true, nil }

type workspaceResolver struct{}

func (workspaceResolver) Tenant(context.Context, core.Identity) (string, bool) {
	return "tea-acme", true
}
func (workspaceResolver) IsMember(context.Context, core.Identity, string) (bool, error) {
	return true, nil
}

// fakeStore is the two tables, in memory.
type fakeStore struct {
	owners  map[string]string
	slots   map[string]store.AvatarUpload // by string(hash)
	avatars map[core.AvatarOwner]store.Avatar
	reads   int
	asked   int // owners the last batch read named
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		owners:  map[string]string{"identity-a": "own-d4a5cbbpdc73dcrkvg00", "identity-b": "own-d4a5cbbpdc73dcrkvgbb"},
		slots:   map[string]store.AvatarUpload{},
		avatars: map[core.AvatarOwner]store.Avatar{},
	}
}

func (f *fakeStore) OwnerIDForSubject(_ context.Context, subject string) (string, error) {
	return f.owners[subject], nil
}

func (f *fakeStore) CreateAvatarUpload(_ context.Context, u store.AvatarUpload) error {
	f.slots[string(u.TokenHash)] = u
	return nil
}

func (f *fakeStore) RedeemAvatarUpload(_ context.Context, hash []byte, now time.Time) (store.AvatarUpload, error) {
	u, ok := f.slots[string(hash)]
	delete(f.slots, string(hash))
	if !ok || !now.Before(u.ExpiresAt) {
		return store.AvatarUpload{}, store.ErrNotFound
	}
	return u, nil
}

func (f *fakeStore) PutAvatar(_ context.Context, a store.Avatar, _ string) error {
	f.avatars[a.Owner] = a
	return nil
}

func (f *fakeStore) DeleteAvatar(_ context.Context, o core.AvatarOwner) (bool, error) {
	_, ok := f.avatars[o]
	delete(f.avatars, o)
	return ok, nil
}

func (f *fakeStore) AvatarByDownloadKey(_ context.Context, key string) (store.Avatar, error) {
	for _, a := range f.avatars {
		if a.DownloadKey == key {
			return a, nil
		}
	}
	return store.Avatar{}, store.ErrNotFound
}

func (f *fakeStore) AvatarDownloadKeys(_ context.Context, owners []core.AvatarOwner) (map[core.AvatarOwner]string, error) {
	f.reads++
	f.asked = len(owners)
	out := map[core.AvatarOwner]string{}
	for _, o := range owners {
		if a, ok := f.avatars[o]; ok {
			out[o] = a.DownloadKey
		}
	}
	return out, nil
}

const ownUserID = "usr-d4a5cbbpdc73dcrkvg00"

var accountA = core.AvatarOwner{ID: "own-d4a5cbbpdc73dcrkvg00", Type: core.AvatarTypeAccount}

func newService(st *fakeStore) *Service {
	return &Service{Base: &core.Base{Authz: allowChecker{}, Workspace: workspaceResolver{}}, Store: st, PublicURL: "https://api.bex.test/"}
}

func person(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: core.MethodSession, Human: true})
}

func encodePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{G: 90, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func encodeJPEG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 128, 128)), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func routes(s *Service) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle(UploadPattern, s.UploadHandler())
	mux.Handle(DownloadPattern, s.DownloadHandler())
	return mux
}

func serve(mux *http.ServeMux, method, url string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url[strings.Index(url, "/v1/"):], bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func codeOf(err error) string {
	var coded *core.CodedError
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}

// Render's own documents (owner-*.js, 2026-11-26), unchanged.
const (
	renderPresign = `query avatarPresignedUpload(
    $avatarOwnerId: String!
    $avatarType: AvatarType!
    $avatarSizeBytes: Int!
  ) {
    avatarPresignedUpload(
      avatarOwnerId: $avatarOwnerId
      avatarType: $avatarType
      avatarSizeBytes: $avatarSizeBytes
    ) {
      url
      verb
    }
  }`
	renderDelete = `mutation deleteAvatar($avatarOwnerId: String!, $avatarType: AvatarType!) {
    deleteAvatar(avatarOwnerId: $avatarOwnerId, avatarType: $avatarType)
  }`
)

func TestRenderDocumentsUploadAndResetAnAvatar(t *testing.T) {
	st := newFakeStore()
	s := newService(st)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: s.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: s.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := person("identity-a")
	img := encodeJPEG(t)
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: renderPresign,
		VariableValues: map[string]any{"avatarOwnerId": ownUserID, "avatarType": "ACCOUNT", "avatarSizeBytes": len(img)}})
	if len(res.Errors) != 0 {
		t.Fatal(res.Errors)
	}
	up := res.Data.(map[string]any)["avatarPresignedUpload"].(map[string]any)
	url := up["url"].(string)
	if up["verb"] != "PUT" || !strings.HasPrefix(url, "https://api.bex.test/v1/avatar-uploads/") {
		t.Fatalf("presigned upload = %v", up)
	}
	// The store keeps the token's hash, never the token.
	token := strings.TrimPrefix(url, "https://api.bex.test/v1/avatar-uploads/")
	if _, kept := st.slots[token]; kept || len(st.slots) != 1 {
		t.Fatalf("slots = %v", st.slots)
	}
	if rec := serve(routes(s), http.MethodPut, url, img); rec.Code != http.StatusOK {
		t.Fatalf("PUT = %d %s", rec.Code, rec.Body)
	}
	if a := st.avatars[accountA]; a.ContentType != "image/jpeg" || !bytes.Equal(a.Content, img) {
		t.Fatalf("stored = %+v", a)
	}
	res = graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: renderDelete,
		VariableValues: map[string]any{"avatarOwnerId": ownUserID, "avatarType": "ACCOUNT"}})
	if len(res.Errors) != 0 || res.Data.(map[string]any)["deleteAvatar"] != true || len(st.avatars) != 0 {
		t.Fatalf("deleteAvatar = %v %v, avatars %v", res.Data, res.Errors, st.avatars)
	}
}

func TestPresignedUploadRefusals(t *testing.T) {
	s := newService(newFakeStore())
	for name, tc := range map[string]struct {
		ctx   context.Context
		owner string
		kind  string
		size  int
		code  string
	}{
		"another person's account": {person("identity-b"), ownUserID, core.AvatarTypeAccount, 10, core.NotFoundCode},
		"an unknown account":       {person("identity-a"), "usr-nobody", core.AvatarTypeAccount, 10, core.NotFoundCode},
		"an API key": {core.WithIdentity(context.Background(), core.Identity{Subject: "key", Method: core.MethodOAuth2, ClientID: "key"}),
			ownUserID, core.AvatarTypeAccount, 10, "AVATAR_PERSON_REQUIRED"},
		"no type":          {person("identity-a"), ownUserID, "", 10, codeType},
		"an empty image":   {person("identity-a"), ownUserID, core.AvatarTypeAccount, 0, codeSize},
		"over 1 MiB":       {person("identity-a"), ownUserID, core.AvatarTypeAccount, store.MaxAvatarBytes + 1, codeSize},
		"workspace, empty": {person("identity-a"), "tea-acme", core.AvatarTypeWorkspace, 0, codeSize},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := s.PresignedUpload(tc.ctx, tc.owner, tc.kind, tc.size)
			if got := codeOf(err); got != tc.code {
				t.Fatalf("code = %q (%v), want %q", got, err, tc.code)
			}
		})
	}
	// The own- spelling of your own id is yours too.
	if _, err := s.PresignedUpload(person("identity-a"), accountA.ID, core.AvatarTypeAccount, 10); err != nil {
		t.Fatalf("own- id: %v", err)
	}
	// Without BEX_API_PUBLIC_URL the URL is a path on the API origin.
	s.PublicURL = ""
	up, err := s.PresignedUpload(person("identity-a"), ownUserID, core.AvatarTypeAccount, 10)
	if err != nil || !strings.HasPrefix(up.URL, "/v1/avatar-uploads/") {
		t.Fatalf("relative upload URL = %v, %v", up, err)
	}
}

// TestUploadWritesOnlyTheSlotsOwner is t008's "a presigned upload cannot be
// confirmed for another user's or workspace's key": the PUT names nothing but
// its token, and the token is bound to the owner it was minted for.
func TestUploadWritesOnlyTheSlotsOwner(t *testing.T) {
	st := newFakeStore()
	s := newService(st)
	mux := routes(s)
	img := encodePNG(t, 4, 4)
	ws, err := s.PresignedUpload(person("identity-a"), "tea-acme", core.AvatarTypeWorkspace, len(img))
	if err != nil {
		t.Fatal(err)
	}
	// A query string or a path naming another owner changes nothing.
	if rec := serve(mux, http.MethodPut, ws.URL+"?avatarOwnerId="+ownUserID+"&avatarType=ACCOUNT", img); rec.Code != http.StatusOK {
		t.Fatalf("PUT = %d %s", rec.Code, rec.Body)
	}
	if _, ok := st.avatars[core.WorkspaceAvatar("tea-acme")]; !ok || len(st.avatars) != 1 {
		t.Fatalf("avatars = %v, want only the workspace's", st.avatars)
	}
	// Spent: a second PUT on the same URL stores nothing.
	if rec := serve(mux, http.MethodPut, ws.URL, encodePNG(t, 4, 4)); rec.Code != http.StatusNotFound {
		t.Fatalf("second PUT = %d, want 404", rec.Code)
	}
	// A made-up token is the same 404 as a spent one.
	if rec := serve(mux, http.MethodPut, "/v1/avatar-uploads/"+strings.Repeat("A", 43), img); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown token = %d, want 404", rec.Code)
	}
	if rec := serve(mux, http.MethodPut, "/v1/avatar-uploads/short", img); rec.Code != http.StatusNotFound {
		t.Fatalf("malformed token = %d, want 404", rec.Code)
	}
}

func TestUploadRefusesWhatIsNotAnAvatar(t *testing.T) {
	st := newFakeStore()
	s := newService(st)
	mux := routes(s)
	huge := encodePNG(t, maxDimension+1, 1)
	for name, body := range map[string][]byte{
		"html":               []byte("<html><script>alert(1)</script></html>"),
		"svg":                []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`),
		"truncated png":      encodePNG(t, 4, 4)[:20],
		"too wide to be one": huge,
	} {
		t.Run(name, func(t *testing.T) {
			up, err := s.PresignedUpload(person("identity-a"), ownUserID, core.AvatarTypeAccount, len(body))
			if err != nil {
				t.Fatal(err)
			}
			if rec := serve(mux, http.MethodPut, up.URL, body); rec.Code != http.StatusBadRequest {
				t.Fatalf("PUT = %d, want 400", rec.Code)
			}
			if len(st.avatars) != 0 {
				t.Fatal("a refused body was stored")
			}
		})
	}
	// A body that is not the size the URL was minted for, either way.
	img := encodePNG(t, 4, 4)
	for _, size := range []int{len(img) - 1, len(img) + 1} {
		up, err := s.PresignedUpload(person("identity-a"), ownUserID, core.AvatarTypeAccount, size)
		if err != nil {
			t.Fatal(err)
		}
		if rec := serve(mux, http.MethodPut, up.URL, img); rec.Code != http.StatusBadRequest {
			t.Fatalf("PUT %d bytes on a %d-byte URL = %d, want 400", len(img), size, rec.Code)
		}
	}
	// An expired URL is gone.
	up, err := s.PresignedUpload(person("identity-a"), ownUserID, core.AvatarTypeAccount, len(img))
	if err != nil {
		t.Fatal(err)
	}
	s.Clock = func() time.Time { return time.Now().Add(UploadTTL + time.Second) }
	if rec := serve(mux, http.MethodPut, up.URL, img); rec.Code != http.StatusNotFound {
		t.Fatalf("expired PUT = %d, want 404", rec.Code)
	}
}

func TestDownloadServesAnInertImage(t *testing.T) {
	st := newFakeStore()
	s := newService(st)
	img := encodePNG(t, 2, 2)
	st.avatars[accountA] = store.Avatar{Owner: accountA, DownloadKey: "k1", ContentType: "image/png", Content: img}
	rec := serve(routes(s), http.MethodGet, "/v1/avatars/k1", nil)
	h := rec.Result().Header
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), img) {
		t.Fatalf("GET = %d (%d bytes)", rec.Code, rec.Body.Len())
	}
	for k, want := range map[string]string{
		"Content-Type":                 "image/png",
		"X-Content-Type-Options":       "nosniff",
		"Content-Security-Policy":      "default-src 'none'; sandbox",
		"Cache-Control":                "public, max-age=31536000, immutable",
		"Cross-Origin-Resource-Policy": "cross-origin",
	} {
		if got := h.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if rec := serve(routes(s), http.MethodGet, "/v1/avatars/nope", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("missing key = %d %v", rec.Code, rec.Result().Header)
	}
}

type member struct{ ID string }

// TestAvatarDownloadFieldsReadInOneQuery is the batch: a list of objects that
// each show an avatar costs one store read, whatever its length.
func TestAvatarDownloadFieldsReadInOneQuery(t *testing.T) {
	st := newFakeStore()
	s := newService(st)
	st.avatars[accountA] = store.Avatar{Owner: accountA, DownloadKey: "k1"}
	memberType := graphql.NewObject(graphql.ObjectConfig{Name: "M", Fields: graphql.Fields{
		"avatarDownload": gqlutil.AvatarDownloadField(func(m member) core.AvatarOwner { return core.AccountAvatar(m.ID) }),
	}})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: graphql.Fields{
		"members": &graphql.Field{Type: graphql.NewList(memberType), Resolve: func(graphql.ResolveParams) (any, error) {
			return []member{{ownUserID}, {"usr-d4a5cbbpdc73dcrkvgbb"}, {"usr-d4a5cbbpdc73dcrkvgcc"}}, nil
		}},
	}})})
	if err != nil {
		t.Fatal(err)
	}
	ctx := core.WithAvatarBatch(context.Background(), core.NewAvatarBatch(s.URLs()))
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `{ members { avatarDownload { url } } a: members { avatarDownload { url } } }`})
	if len(res.Errors) != 0 {
		t.Fatal(res.Errors)
	}
	list := res.Data.(map[string]any)["members"].([]any)
	first, _ := list[0].(map[string]any)["avatarDownload"].(map[string]any)
	if first["url"] != "https://api.bex.test/v1/avatars/k1" || list[1].(map[string]any)["avatarDownload"] != nil {
		t.Fatalf("members = %v", list)
	}
	if st.reads != 1 || st.asked != 3 {
		t.Fatalf("%d store reads naming %d owners for six avatar fields of three people, want 1 naming 3", st.reads, st.asked)
	}
}
