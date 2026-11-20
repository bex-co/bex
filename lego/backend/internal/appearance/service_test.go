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

package appearance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

type allowChecker struct{}

func (allowChecker) Check(context.Context, string, string, string) (bool, error) { return true, nil }

type workspaceResolver struct{}

func (workspaceResolver) Tenant(context.Context, core.Identity) (string, bool) {
	return "tea-own", true
}
func (workspaceResolver) IsMember(context.Context, core.Identity, string) (bool, error) {
	return true, nil
}

// fakeStore keeps one row per subject, as user_theme_settings does.
type fakeStore struct {
	rows   map[string]store.UserThemeSettings
	owners map[string]string
	err    error
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		rows:   map[string]store.UserThemeSettings{},
		owners: map[string]string{"identity-a": "own-d4a5cbbpdc73dcrkvg00", "identity-b": "own-d4a5cbbpdc73dcrkvgbb"},
	}
}

func (f *fakeStore) GetUserThemeSettings(_ context.Context, subject string) (store.UserThemeSettings, error) {
	if row, ok := f.rows[subject]; ok {
		return row, nil
	}
	return store.DefaultUserThemeSettings, nil
}

func (f *fakeStore) set(subject string, apply func(*store.UserThemeSettings)) (store.UserThemeSettings, error) {
	if f.err != nil {
		return store.UserThemeSettings{}, f.err
	}
	row, _ := f.GetUserThemeSettings(context.Background(), subject)
	apply(&row)
	f.rows[subject] = row
	return row, nil
}

func (f *fakeStore) SetUserThemeSetting(_ context.Context, subject, value string) (store.UserThemeSettings, error) {
	return f.set(subject, func(r *store.UserThemeSettings) { r.ThemeSetting = value })
}

func (f *fakeStore) SetUserLogThemeSetting(_ context.Context, subject, value string) (store.UserThemeSettings, error) {
	return f.set(subject, func(r *store.UserThemeSettings) { r.LogThemeSetting = value })
}

func (f *fakeStore) OwnerIDForSubject(_ context.Context, subject string) (string, error) {
	return f.owners[subject], nil
}

func newService(st Store) *Service {
	return &Service{Base: &core.Base{Authz: allowChecker{}, Workspace: workspaceResolver{}}, Store: st}
}

func person(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: core.MethodSession, Human: true})
}

const ownUserID = "usr-d4a5cbbpdc73dcrkvg00"

func schemaOf(t *testing.T, s *Service) graphql.Schema {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: s.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: s.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func gql(t *testing.T, schema graphql.Schema, ctx context.Context, doc string, vars map[string]any) *graphql.Result {
	t.Helper()
	return graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: doc, VariableValues: vars})
}

func data(t *testing.T, result *graphql.Result) string {
	t.Helper()
	if len(result.Errors) != 0 {
		t.Fatalf("GraphQL errors: %v", result.Errors)
	}
	out, err := json.Marshal(result.Data)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// The documents Render's dashboard sends (user-*.js, 2026-11-20), unchanged.
const (
	renderUserQuery = `query user { user { id themeSetting logThemeSetting } }`

	renderUpdateTheme = `mutation updateUserThemeSetting($id: String!, $themeSetting: ThemeSetting!) {
    updateUserThemeSetting(id: $id, themeSetting: $themeSetting) {
      id
      themeSetting
    }
  }`

	renderUpdateLogTheme = `mutation updateUserLogThemeSetting($id: String!, $logThemeSetting: LogThemeSetting!) {
    updateUserLogThemeSetting(id: $id, logThemeSetting: $logThemeSetting) {
      id
      logThemeSetting
    }
  }`
)

func TestRenderDocumentsReadAndWriteTheAccountsThemes(t *testing.T) {
	st := newFakeStore()
	schema := schemaOf(t, newService(st))
	ctx := person("identity-a")

	if got, want := data(t, gql(t, schema, ctx, renderUserQuery, nil)),
		`{"user":{"id":"`+ownUserID+`","logThemeSetting":"MATCH_APP","themeSetting":"SYSTEM"}}`; got != want {
		t.Fatalf("never chose = %s, want %s", got, want)
	}
	if got, want := data(t, gql(t, schema, ctx, renderUpdateTheme, map[string]any{"id": ownUserID, "themeSetting": "DARK_HIGH_CONTRAST"})),
		`{"updateUserThemeSetting":{"id":"`+ownUserID+`","themeSetting":"DARK_HIGH_CONTRAST"}}`; got != want {
		t.Fatalf("updateUserThemeSetting = %s, want %s", got, want)
	}
	if got, want := data(t, gql(t, schema, ctx, renderUpdateLogTheme, map[string]any{"id": ownUserID, "logThemeSetting": "ALWAYS_LIGHT"})),
		`{"updateUserLogThemeSetting":{"id":"`+ownUserID+`","logThemeSetting":"ALWAYS_LIGHT"}}`; got != want {
		t.Fatalf("updateUserLogThemeSetting = %s, want %s", got, want)
	}
	// Each write kept the other value.
	if got, want := data(t, gql(t, schema, ctx, renderUserQuery, nil)),
		`{"user":{"id":"`+ownUserID+`","logThemeSetting":"ALWAYS_LIGHT","themeSetting":"DARK_HIGH_CONTRAST"}}`; got != want {
		t.Fatalf("read back = %s, want %s", got, want)
	}
	// Another account reads its own defaults.
	if got := data(t, gql(t, schema, person("identity-b"), renderUserQuery, nil)); !strings.Contains(got, `"themeSetting":"SYSTEM"`) {
		t.Fatalf("another account = %s", got)
	}
}

func TestGraphQLRefusesAnotherUsersIDAnUnknownValueAndAnAPIKey(t *testing.T) {
	st := newFakeStore()
	schema := schemaOf(t, newService(st))
	ctx := person("identity-a")

	other := gql(t, schema, ctx, renderUpdateTheme, map[string]any{"id": "usr-d4a5cbbpdc73dcrkvgbb", "themeSetting": "DARK"})
	if len(other.Errors) == 0 || !strings.Contains(other.Errors[0].Message, "user not found") {
		t.Fatalf("another user's id = %v, want user not found", other.Errors)
	}
	if _, wrote := st.rows["identity-a"]; wrote {
		t.Fatal("a refused write was stored")
	}
	if _, wrote := st.rows["identity-b"]; wrote {
		t.Fatal("another user's id wrote that user's row")
	}

	for _, doc := range []string{
		`mutation { updateUserThemeSetting(id: "` + ownUserID + `", themeSetting: HIGH_CONTRAST) { id } }`,
		`mutation { updateUserLogThemeSetting(id: "` + ownUserID + `", logThemeSetting: DARK) { id } }`,
	} {
		if result := gql(t, schema, ctx, doc, nil); len(result.Errors) == 0 {
			t.Fatalf("%s: an unknown value was accepted", doc)
		}
	}

	key := core.WithIdentity(context.Background(), core.Identity{Subject: "rnd-key", ClientID: "rnd-key", Method: core.MethodOAuth2})
	if result := gql(t, schema, key, renderUserQuery, nil); len(result.Errors) == 0 || !strings.Contains(result.Errors[0].Message, "signed-in person") {
		t.Fatalf("an API key read = %v, want refused", result.Errors)
	}
	if result := gql(t, schema, context.Background(), renderUserQuery, nil); len(result.Errors) == 0 {
		t.Fatal("an unauthenticated read was answered")
	}
}

func serve(s *Service, ctx context.Context, method, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	s.RegisterREST(mux)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(method, "/v1/users/theme-settings", strings.NewReader(body)).WithContext(ctx))
	return recorder
}

func TestRESTReadsAndPatchesEitherValue(t *testing.T) {
	st := newFakeStore()
	s := newService(st)
	ctx := person("identity-a")

	if got := serve(s, ctx, http.MethodGet, ""); got.Code != http.StatusOK ||
		strings.TrimSpace(got.Body.String()) != `{"themeSetting":"SYSTEM","logThemeSetting":"MATCH_APP"}` {
		t.Fatalf("GET defaults = %d %s", got.Code, got.Body)
	}
	if got := serve(s, ctx, http.MethodPatch, `{"logThemeSetting":"ALWAYS_DARK"}`); got.Code != http.StatusOK ||
		strings.TrimSpace(got.Body.String()) != `{"themeSetting":"SYSTEM","logThemeSetting":"ALWAYS_DARK"}` {
		t.Fatalf("PATCH log theme = %d %s", got.Code, got.Body)
	}
	if got := serve(s, ctx, http.MethodPatch, `{"themeSetting":"LIGHT_HIGH_CONTRAST"}`); got.Code != http.StatusOK ||
		strings.TrimSpace(got.Body.String()) != `{"themeSetting":"LIGHT_HIGH_CONTRAST","logThemeSetting":"ALWAYS_DARK"}` {
		t.Fatalf("PATCH theme = %d %s", got.Code, got.Body)
	}
	if got := serve(s, ctx, http.MethodPatch, `{}`); got.Code != http.StatusOK ||
		strings.TrimSpace(got.Body.String()) != `{"themeSetting":"LIGHT_HIGH_CONTRAST","logThemeSetting":"ALWAYS_DARK"}` {
		t.Fatalf("empty PATCH = %d %s", got.Code, got.Body)
	}

	for _, body := range []string{
		`{"themeSetting":"dark"}`,
		`{"logThemeSetting":"DARK"}`,
		`{"themeSetting":"DARK","highContrast":true}`,
		`{"themeSetting":null,"logThemeSetting":"ALWAYS_LIGHT"} {}`,
	} {
		got := serve(s, ctx, http.MethodPatch, body)
		if got.Code != http.StatusBadRequest {
			t.Fatalf("PATCH %s = %d %s, want 400", body, got.Code, got.Body)
		}
	}
	if row := st.rows["identity-a"]; row.ThemeSetting != store.ThemeSettingLightHighContrast || row.LogThemeSetting != store.LogThemeSettingAlwaysDark {
		t.Fatalf("a refused PATCH changed the row: %+v", row)
	}

	key := core.WithIdentity(context.Background(), core.Identity{Subject: "rnd-key", ClientID: "rnd-key", Method: core.MethodOAuth2})
	if got := serve(s, key, http.MethodGet, ""); got.Code != http.StatusForbidden {
		t.Fatalf("an API key's GET = %d %s, want 403", got.Code, got.Body)
	}
	if got := serve(s, context.Background(), http.MethodPatch, `{"themeSetting":"DARK"}`); got.Code != http.StatusForbidden && got.Code != http.StatusUnauthorized {
		t.Fatalf("an unauthenticated PATCH = %d %s", got.Code, got.Body)
	}
}

func TestAWriteWhileTheAccountIsBeingDeletedIsRefused(t *testing.T) {
	st := newFakeStore()
	st.err = store.ErrAccountDeletionPending
	got := serve(newService(st), person("identity-a"), http.MethodPatch, `{"themeSetting":"DARK"}`)
	if got.Code != http.StatusConflict {
		t.Fatalf("PATCH during deletion = %d %s, want 409", got.Code, got.Body)
	}
}

func TestUnconfiguredStoreIsUnavailable(t *testing.T) {
	got := serve(newService(nil), person("identity-a"), http.MethodGet, "")
	if got.Code != http.StatusServiceUnavailable || !strings.Contains(got.Body.String(), codeUnavailable) {
		t.Fatalf("no store = %d %s", got.Code, got.Body)
	}
}

// The same verbs over a real Postgres: REST and GraphQL read and write one row.
func TestAppearanceOverPostgresPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	st := store.NewPGStore(pool)
	const subject = "identity-appearance-pg"
	if _, err := pool.Exec(context.Background(), `DELETE FROM user_theme_settings WHERE subject = $1`, subject); err != nil {
		t.Fatal(err)
	}
	s := newService(st)
	ctx := person(subject)
	schema := schemaOf(t, s)

	user := data(t, gql(t, schema, ctx, renderUserQuery, nil))
	var read struct {
		User User `json:"user"`
	}
	if err := json.Unmarshal([]byte(user), &read); err != nil || !strings.HasPrefix(read.User.ID, "usr-") {
		t.Fatalf("user = %s (%v), want a usr- id", user, err)
	}
	if got := serve(s, ctx, http.MethodPatch, `{"themeSetting":"DARK"}`); got.Code != http.StatusOK {
		t.Fatalf("REST PATCH = %d %s", got.Code, got.Body)
	}
	data(t, gql(t, schema, ctx, renderUpdateLogTheme, map[string]any{"id": read.User.ID, "logThemeSetting": "ALWAYS_LIGHT"}))
	if got := serve(s, ctx, http.MethodGet, ""); strings.TrimSpace(got.Body.String()) != `{"themeSetting":"DARK","logThemeSetting":"ALWAYS_LIGHT"}` {
		t.Fatalf("REST GET after both writes = %d %s", got.Code, got.Body)
	}
}
