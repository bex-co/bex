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

package agentsessions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestUnifiedLineDiff(t *testing.T) {
	cases := []struct{ name, old, new, want string }{
		{"equal", "a\nb\n", "a\nb\n", ""},
		{"first version is all additions", "", "a\nb\n", "@@ -0,0 +1,2 @@\n+a\n+b\n"},
		{"everything removed", "a\nb\n", "", "@@ -1,2 +0,0 @@\n-a\n-b\n"},
		{"one changed line", "a\nb\nc\n", "a\nB\nc\n", "@@ -1,3 +1,3 @@\n a\n-b\n+B\n c\n"},
		{"insertion keeps context", "1\n2\n3\n4\n5\n6\n7\n8\n9\n", "1\n2\n3\n4\nX\n5\n6\n7\n8\n9\n",
			"@@ -2,6 +2,7 @@\n 2\n 3\n 4\n+X\n 5\n 6\n 7\n"},
		{"distant changes are two hunks", "a\n1\n2\n3\n4\n5\n6\n7\n8\nz\n", "A\n1\n2\n3\n4\n5\n6\n7\n8\nZ\n",
			"@@ -1,4 +1,4 @@\n-a\n+A\n 1\n 2\n 3\n@@ -7,4 +7,4 @@\n 6\n 7\n 8\n-z\n+Z\n"},
		{"no trailing newline compares by lines", "a\nb", "a\nb\nc", "@@ -1,2 +1,3 @@\n a\n b\n+c\n"},
		{"adding only a trailing newline is no change", "a\nb", "a\nb\n", ""},
		{"a single line from nothing without newline", "", "a", "@@ -0,0 +1,1 @@\n+a\n"},
		{"CRLF line endings are part of the line", "a\r\nb\r\n", "a\r\nB\r\n", "@@ -1,2 +1,2 @@\n a\r\n-b\r\n+B\r\n"},
		{"CRLF against LF differs on every line", "a\r\nb\r\n", "a\nb\n", "@@ -1,2 +1,2 @@\n-a\r\n-b\r\n+a\n+b\n"},
		{"a blank line is a line", "a\n\nb\n", "a\nb\n", "@@ -1,3 +1,2 @@\n a\n-\n b\n"},
		{"frontmatter fences diff like any line", "---\nx\n---\n", "---\ny\n---\n", "@@ -1,3 +1,3 @@\n ---\n-x\n+y\n ---\n"},
	}
	for _, c := range cases {
		if got := unifiedLineDiff(c.old, c.new); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
	// A middle too large for the matching table is still a correct (whole
	// replacement) diff.
	// Common lines around a too-large middle stay context; only the middle is
	// replaced.
	huge := func(c string) string { return "head\n" + strings.Repeat(c+"\n", 2500) + "tail\n" }
	got := unifiedLineDiff(huge("p"), huge("q"))
	if !strings.HasPrefix(got, "@@ -1,2502 +1,2502 @@\n head\n-p\n") {
		t.Errorf("large middle diff = %.80q", got)
	}
	if !strings.HasSuffix(got, "+q\n tail\n") || strings.Count(got, "\n-p") != 2500 || strings.Count(got, "\n+q") != 2500 {
		t.Errorf("large middle diff: %d removed, %d added, tail %q", strings.Count(got, "\n-p"), strings.Count(got, "\n+q"), got[len(got)-12:])
	}
	// Just under the table limit the real LCS runs: an inserted line among 2000
	// shows as one addition, not a replacement.
	many := strings.Repeat("l\n", 1000) + "m\n" + strings.Repeat("l\n", 1000)
	if d := unifiedLineDiff(many, strings.Repeat("l\n", 1000)+"m\nX\n"+strings.Repeat("l\n", 1000)); strings.Count(d, "\n+") != 1 || strings.Count(d, "\n-") != 0 {
		t.Errorf("a single insertion among 2000 lines = %.120q", d)
	}
	big := strings.Repeat("x\n", 3000)
	other := strings.Repeat("y\n", 3000)
	if got := unifiedLineDiff(big, other); !strings.HasPrefix(got, "@@ -1,3000 +1,3000 @@\n-x\n") {
		t.Errorf("large diff = %.60q", got)
	}
}

type skillVersionSurfaces struct {
	t   *testing.T
	svc *Service
	mux *http.ServeMux
}

func newSkillVersionSurfaces(t *testing.T) *skillVersionSurfaces {
	svc := environmentService("tea-a", newFakeEnvironments())
	svc.Skills = newFakeSkills()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	return &skillVersionSurfaces{t: t, svc: svc, mux: mux}
}

func (s *skillVersionSurfaces) rest(path, who string) (int, map[string]any) {
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil).WithContext(caller(who)))
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func (s *skillVersionSurfaces) gql(query, who string) (map[string]any, string) {
	result := graphql.Do(graphql.Params{Schema: environmentSchema(s.t, s.svc), Context: caller(who), RequestString: query})
	if len(result.Errors) > 0 {
		code, _ := result.Errors[0].Extensions["code"].(string)
		return nil, code
	}
	data, _ := result.Data.(map[string]any)
	return data, ""
}

func (s *skillVersionSurfaces) mcp(tool string, args map[string]any, who string) (map[string]any, string) {
	res, err := newMCPClient(s.t, caller(who), s.svc).CallTool(caller(who), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		s.t.Fatalf("MCP %s: %v", tool, err)
	}
	if res.IsError {
		return nil, res.Content[0].(*mcp.TextContent).Text
	}
	out, _ := res.StructuredContent.(map[string]any)
	return out, ""
}

// Three saves leave three versions that REST, GraphQL and MCP return with the
// same fields; a version carries its content and the diff against the
// previous one; restoring is an ordinary update that adds a version.
func TestAgentSkillVersionsAreReadableOnEverySurface(t *testing.T) {
	s := newSkillVersionSurfaces(t)
	ctx := caller("admin")
	created, err := s.svc.CreateAgentSkill(ctx, "tea-a", AgentSkillInput{Content: str(deploySkill)})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.svc.UpdateAgentSkill(ctx, "tea-a", created.ID, AgentSkillInput{Body: str("Run `make deploy`.\nThen check the dashboard.\n"), IfVersion: &created.Version})
	if err != nil {
		t.Fatal(err)
	}
	third, err := s.svc.UpdateAgentSkill(ctx, "tea-a", "deploy-staging", AgentSkillInput{Description: str("Deploy to staging, carefully")})
	if err != nil || third.Version != 3 {
		t.Fatalf("third save = %+v err=%v", third, err)
	}

	status, list := s.rest("/v1/agent-skills/"+created.ID+"/versions?ownerId=tea-a", "dev")
	versions, _ := list["versions"].([]any)
	if status != http.StatusOK || len(versions) != 3 || list["skillId"] != created.ID || list["ownerId"] != "tea-a" || list["nextCursor"] != nil {
		t.Fatalf("REST list = %d %v", status, list)
	}
	for i, want := range []struct {
		version float64
		message string
		current bool
	}{{3, "Update skills/deploy-staging/SKILL.md", true}, {2, "Update skills/deploy-staging/SKILL.md", false}, {1, "Add skills/deploy-staging/SKILL.md", false}} {
		v := versions[i].(map[string]any)
		short, _ := v["shortId"].(string)
		if v["version"] != want.version || v["message"] != want.message || v["current"] != want.current || v["createdBy"] != "admin" ||
			len(short) != 7 || v["content"] != nil || v["diff"] != nil || v["createdAt"] == nil {
			t.Fatalf("version %d in the list = %v", i, v)
		}
	}

	// A single version: full content and the diff against the previous one.
	status, v2 := s.rest("/v1/agent-skills/deploy-staging/versions/2?ownerId=tea-a", "dev")
	if status != http.StatusOK || v2["version"] != float64(2) || v2["previousVersion"] != float64(1) || v2["current"] != false ||
		v2["content"] != storedContent(t, s, created.ID, 2) {
		t.Fatalf("REST version 2 = %d %v", status, v2)
	}
	wantDiff := "@@ -3,4 +3,5 @@\n description: How to deploy to staging\n argument-hint: <service>\n ---\n-Run `make deploy SERVICE=$1` for $ARGUMENTS.\n+Run `make deploy`.\n+Then check the dashboard.\n"
	if v2["diff"] != wantDiff {
		t.Fatalf("diff of version 2 = %q, want %q", v2["diff"], wantDiff)
	}
	status, v1 := s.rest("/v1/agent-skills/"+created.ID+"/versions/1?ownerId=tea-a", "dev")
	if status != http.StatusOK || v1["previousVersion"] != nil || !strings.HasPrefix(v1["diff"].(string), "@@ -0,0 +1,") || strings.Contains(v1["diff"].(string), "\n-") {
		t.Fatalf("REST version 1 = %d %v", status, v1)
	}
	// The current version reads as the live skill does.
	status, v3 := s.rest("/v1/agent-skills/"+created.ID+"/versions/3?ownerId=tea-a", "dev")
	if status != http.StatusOK || v3["current"] != true || v3["content"] != third.Content {
		t.Fatalf("REST version 3 = %d %v", status, v3)
	}

	// GraphQL and MCP return the same records.
	data, code := s.gql(fmt.Sprintf(`{ agentSkillVersions(ownerId: "tea-a", id: %q) { skillId ownerId nextCursor versions { skillId ownerId version shortId message createdBy createdAt current content diff previousVersion } } }`, created.ID), "dev")
	if code != "" {
		t.Fatalf("GraphQL list code %s", code)
	}
	gl, _ := data["agentSkillVersions"].(map[string]any)
	gv, _ := gl["versions"].([]any)
	if len(gv) != 3 || gl["nextCursor"] != nil {
		t.Fatalf("GraphQL list = %v", gl)
	}
	for i := range gv {
		g, r := gv[i].(map[string]any), versions[i].(map[string]any)
		for _, k := range []string{"skillId", "ownerId", "version", "shortId", "message", "createdBy", "current"} {
			if fmt.Sprint(g[k]) != fmt.Sprint(r[k]) {
				t.Errorf("GraphQL %s of version %d = %v, REST %v", k, i, g[k], r[k])
			}
		}
		if g["content"] != nil || g["diff"] != nil {
			t.Errorf("GraphQL list carries content: %v", g)
		}
	}
	data, code = s.gql(fmt.Sprintf(`{ agentSkillVersion(ownerId: "tea-a", id: "deploy-staging", version: 2) { version shortId current content diff previousVersion } }`), "dev")
	gv2, _ := data["agentSkillVersion"].(map[string]any)
	if code != "" || gv2["content"] != v2["content"] || gv2["diff"] != v2["diff"] || gv2["shortId"] != v2["shortId"] || fmt.Sprint(gv2["previousVersion"]) != "1" {
		t.Fatalf("GraphQL version 2 = %v code=%s", gv2, code)
	}
	data, _ = s.gql(fmt.Sprintf(`{ agentSkillVersion(ownerId: "tea-a", id: %q, version: 1) { previousVersion diff } }`, created.ID), "dev")
	if gv1, _ := data["agentSkillVersion"].(map[string]any); gv1["previousVersion"] != nil || gv1["diff"] != v1["diff"] {
		t.Fatalf("GraphQL version 1 = %v", gv1)
	}

	ml, msg := s.mcp("list_agent_skill_versions", map[string]any{"ownerId": "tea-a", "skill": created.ID}, "dev")
	mv, _ := ml["versions"].([]any)
	if msg != "" || len(mv) != 3 {
		t.Fatalf("MCP list = %v %s", ml, msg)
	}
	for i := range mv {
		m, r := mv[i].(map[string]any), versions[i].(map[string]any)
		for _, k := range []string{"skillId", "ownerId", "version", "shortId", "message", "createdBy", "createdAt", "current"} {
			if fmt.Sprint(m[k]) != fmt.Sprint(r[k]) {
				t.Errorf("MCP %s of version %d = %v, REST %v", k, i, m[k], r[k])
			}
		}
	}
	mget, msg := s.mcp("get_agent_skill_version", map[string]any{"ownerId": "tea-a", "skill": "deploy-staging", "version": 2}, "dev")
	if msg != "" || mget["content"] != v2["content"] || mget["diff"] != v2["diff"] || mget["shortId"] != v2["shortId"] || mget["previousVersion"] != float64(1) {
		t.Fatalf("MCP version 2 = %v %s", mget, msg)
	}

	// Restore = read an old version, then an ordinary update with ifVersion.
	restored, err := s.svc.UpdateAgentSkill(ctx, "tea-a", created.ID, AgentSkillInput{Content: str(v2["content"].(string)), IfVersion: &third.Version})
	if err != nil || restored.Version != 4 || restored.Content != second.Content {
		t.Fatalf("restore = %+v err=%v", restored, err)
	}
	_, list = s.rest("/v1/agent-skills/"+created.ID+"/versions?ownerId=tea-a", "dev")
	versions, _ = list["versions"].([]any)
	if len(versions) != 4 || versions[0].(map[string]any)["current"] != true || versions[1].(map[string]any)["current"] != false {
		t.Fatalf("history after a restore = %v", list)
	}
	_, old := s.rest("/v1/agent-skills/"+created.ID+"/versions/2?ownerId=tea-a", "dev")
	if old["content"] != v2["content"] || old["shortId"] != v2["shortId"] {
		t.Fatalf("version 2 changed by the restore: %v", old)
	}
}

// storedContent is the content the store holds for a version.
func storedContent(t *testing.T, s *skillVersionSurfaces, id string, version int) string {
	t.Helper()
	v, err := s.svc.Skills.GetAgentSkillVersion(context.Background(), id, version)
	if err != nil {
		t.Fatal(err)
	}
	return v.Content
}

func TestAgentSkillVersionPaging(t *testing.T) {
	s := newSkillVersionSurfaces(t)
	ctx := caller("admin")
	created, err := s.svc.CreateAgentSkill(ctx, "tea-a", AgentSkillInput{Content: str(deploySkill)})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if _, err := s.svc.UpdateAgentSkill(ctx, "tea-a", created.ID, AgentSkillInput{Body: str(fmt.Sprintf("body %d\n", i))}); err != nil {
			t.Fatal(err)
		}
	}
	var seen []float64
	cursor := ""
	for page := 0; page < 5; page++ {
		path := "/v1/agent-skills/" + created.ID + "/versions?ownerId=tea-a&limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		status, got := s.rest(path, "dev")
		list, _ := got["versions"].([]any)
		if status != http.StatusOK || len(list) == 0 || len(list) > 2 {
			t.Fatalf("page %d = %d %v", page, status, got)
		}
		for _, v := range list {
			seen = append(seen, v.(map[string]any)["version"].(float64))
		}
		next, _ := got["nextCursor"].(string)
		if next == "" {
			break
		}
		cursor = next
	}
	if fmt.Sprint(seen) != "[5 4 3 2 1]" {
		t.Fatalf("paged versions = %v, want 5..1 once each", seen)
	}
	// MCP and GraphQL page by the same cursor.
	out, _ := s.mcp("list_agent_skill_versions", map[string]any{"ownerId": "tea-a", "skill": "deploy-staging", "limit": 2, "cursor": "3"}, "dev")
	if mv, _ := out["versions"].([]any); len(mv) != 2 || mv[0].(map[string]any)["version"] != float64(2) || out["nextCursor"] != nil {
		t.Fatalf("MCP page = %v", out)
	}
	if data, _ := s.gql(`{ agentSkillVersions(ownerId: "tea-a", id: "deploy-staging", limit: 1, cursor: "2") { versions { version } nextCursor } }`, "dev"); data == nil ||
		fmt.Sprint(data["agentSkillVersions"]) != "map[nextCursor:<nil> versions:[map[version:1]]]" {
		t.Fatalf("GraphQL page = %v", data)
	}
	if status, _ := s.rest("/v1/agent-skills/"+created.ID+"/versions?ownerId=tea-a&cursor=abc", "dev"); status != http.StatusBadRequest {
		t.Fatalf("a bad cursor = %d", status)
	}
	if status, _ := s.rest("/v1/agent-skills/"+created.ID+"/versions?ownerId=tea-a&limit=0", "dev"); status != http.StatusBadRequest {
		t.Fatalf("limit=0 = %d", status)
	}
}

// An unknown skill and an unknown version are named 404s on every surface; a
// viewer, who cannot read the skill, cannot read its history.
func TestAgentSkillVersionErrorsAndAuthorization(t *testing.T) {
	s := newSkillVersionSurfaces(t)
	created, err := s.svc.CreateAgentSkill(caller("admin"), "tea-a", AgentSkillInput{Content: str(deploySkill)})
	if err != nil {
		t.Fatal(err)
	}
	missing := "skl-doesnotexist"
	for _, tc := range []struct {
		name, path, gql, tool, code string
		args                        map[string]any
	}{
		{"unknown skill, list", "/v1/agent-skills/" + missing + "/versions?ownerId=tea-a",
			fmt.Sprintf(`{ agentSkillVersions(ownerId: "tea-a", id: %q) { skillId } }`, missing),
			"list_agent_skill_versions", "AGENT_SKILL_NOT_FOUND", map[string]any{"ownerId": "tea-a", "skill": missing}},
		{"unknown skill by name, get", "/v1/agent-skills/nope/versions/1?ownerId=tea-a",
			`{ agentSkillVersion(ownerId: "tea-a", id: "nope", version: 1) { version } }`,
			"get_agent_skill_version", "AGENT_SKILL_NOT_FOUND", map[string]any{"ownerId": "tea-a", "skill": "nope", "version": 1}},
		{"unknown version", "/v1/agent-skills/" + created.ID + "/versions/9?ownerId=tea-a",
			fmt.Sprintf(`{ agentSkillVersion(ownerId: "tea-a", id: %q, version: 9) { version } }`, created.ID),
			"get_agent_skill_version", "AGENT_SKILL_VERSION_NOT_FOUND", map[string]any{"ownerId": "tea-a", "skill": created.ID, "version": 9}},
	} {
		status, body := s.rest(tc.path, "dev")
		if status != http.StatusNotFound || !strings.Contains(fmt.Sprint(body), tc.code) {
			t.Errorf("%s: REST = %d %v, want 404 %s", tc.name, status, body, tc.code)
		}
		if _, code := s.gql(tc.gql, "dev"); code != tc.code {
			t.Errorf("%s: GraphQL code = %q, want %s", tc.name, code, tc.code)
		}
		if _, msg := s.mcp(tc.tool, tc.args, "dev"); !strings.Contains(msg, tc.code) {
			t.Errorf("%s: MCP error = %q, want %s", tc.name, msg, tc.code)
		}
	}
	if status, _ := s.rest("/v1/agent-skills/"+created.ID+"/versions/zero?ownerId=tea-a", "dev"); status != http.StatusBadRequest {
		t.Errorf("a non-numeric version = %d", status)
	}

	if status, _ := s.rest("/v1/agent-skills/"+created.ID+"/versions?ownerId=tea-a", "viewer"); status != http.StatusForbidden {
		t.Errorf("a viewer listed versions = %d", status)
	}
	if status, _ := s.rest("/v1/agent-skills/"+created.ID+"/versions/1?ownerId=tea-a", "viewer"); status != http.StatusForbidden {
		t.Errorf("a viewer read a version = %d", status)
	}
	if _, err := s.svc.AgentSkillVersion(caller("viewer"), "tea-a", "deploy-staging", 1); err == nil {
		t.Error("a viewer read a version through the service")
	}
	if _, msg := s.mcp("get_agent_skill_version", map[string]any{"ownerId": "tea-a", "skill": created.ID, "version": 1}, "viewer"); msg == "" {
		t.Error("a viewer read a version over MCP")
	}
	// A developer, who can read the skill, can read its history; the history
	// has no write operation to refuse.
	if status, _ := s.rest("/v1/agent-skills/"+created.ID+"/versions/1?ownerId=tea-a", "dev"); status != http.StatusOK {
		t.Errorf("a developer read a version = %d", status)
	}
}

// A refused save (stale ifVersion) keeps no new version.
func TestRefusedSkillSaveKeepsNoVersion(t *testing.T) {
	s := newSkillVersionSurfaces(t)
	ctx := caller("admin")
	created, err := s.svc.CreateAgentSkill(ctx, "tea-a", AgentSkillInput{Content: str(deploySkill)})
	if err != nil {
		t.Fatal(err)
	}
	stale := 7
	if _, err := s.svc.UpdateAgentSkill(ctx, "tea-a", created.ID, AgentSkillInput{Body: str("x"), IfVersion: &stale}); err == nil {
		t.Fatal("a stale ifVersion was accepted")
	}
	list, err := s.svc.AgentSkillVersions(ctx, "tea-a", created.ID, "", 0)
	if err != nil || len(list.Versions) != 1 || !list.Versions[0].Current {
		t.Fatalf("history after a refused save = %+v err=%v", list, err)
	}
}

// A viewer is refused on GraphQL like on REST, the service and MCP (reading
// the history needs can_create), and a developer reads the same record there.
func TestAgentSkillVersionsDeniedToViewersOnEverySurface(t *testing.T) {
	s := newSkillVersionSurfaces(t)
	created, err := s.svc.CreateAgentSkill(caller("admin"), "tea-a", AgentSkillInput{Content: str(deploySkill)})
	if err != nil {
		t.Fatal(err)
	}
	list := fmt.Sprintf(`{ agentSkillVersions(ownerId: "tea-a", id: %q) { versions { version } } }`, created.ID)
	one := fmt.Sprintf(`{ agentSkillVersion(ownerId: "tea-a", id: %q, version: 1) { content } }`, created.ID)
	for _, q := range []string{list, one} {
		if data, code := s.gql(q, "viewer"); data != nil || code == "" || code == "AGENT_SKILL_NOT_FOUND" {
			t.Errorf("a viewer's GraphQL read of %.40s = %v code=%q, want a refusal", q, data, code)
		}
		if data, code := s.gql(q, "dev"); data == nil || code != "" {
			t.Errorf("a developer's GraphQL read of %.40s = %v code=%q", q, data, code)
		}
	}
	if _, msg := s.mcp("list_agent_skill_versions", map[string]any{"ownerId": "tea-a", "skill": created.ID}, "viewer"); msg == "" {
		t.Error("a viewer listed versions over MCP")
	}
	// A refusal and a "not found" must not be confused: the same viewer gets
	// the same answer for a version that does not exist.
	_, realCode := s.gql(one, "viewer")
	_, missingCode := s.gql(fmt.Sprintf(`{ agentSkillVersion(ownerId: "tea-a", id: %q, version: 99) { content } }`, created.ID), "viewer")
	if realCode != missingCode {
		t.Errorf("a viewer learns whether version 99 exists: %q vs %q", realCode, missingCode)
	}
}

// The current version always reads as the live skill, and a refused or empty
// save changes nothing the history shows: a toggle-only save is a version with
// an empty diff.
func TestAgentSkillToggleIsAVersionWithAnEmptyDiff(t *testing.T) {
	s := newSkillVersionSurfaces(t)
	ctx := caller("admin")
	created, err := s.svc.CreateAgentSkill(ctx, "tea-a", AgentSkillInput{Content: str(deploySkill)})
	if err != nil {
		t.Fatal(err)
	}
	off := false
	if _, err := s.svc.UpdateAgentSkill(ctx, "tea-a", created.ID, AgentSkillInput{Enabled: &off}); err != nil {
		t.Fatal(err)
	}
	status, v2 := s.rest("/v1/agent-skills/"+created.ID+"/versions/2?ownerId=tea-a", "dev")
	if status != http.StatusOK || v2["current"] != true || v2["previousVersion"] != float64(1) || v2["diff"] != nil || v2["content"] == "" {
		t.Fatalf("toggle version = %d %v", status, v2)
	}
	if _, v1 := s.rest("/v1/agent-skills/"+created.ID+"/versions/1?ownerId=tea-a", "dev"); v1["content"] != v2["content"] || v1["shortId"] == v2["shortId"] {
		t.Fatalf("toggle changed the content or kept the short id: %v vs %v", v1, v2)
	}
}

// Where the Go renderer and the 0210 backfill agree, a skill's description is
// written identically; where they differ the difference is the documented one
// (store.backfillDescriptions pins the SQL side of this table).
func TestRenderedDescriptionsMatchTheMigrationBackfill(t *testing.T) {
	for _, c := range []struct{ description, want string }{
		{"How to deploy", "How to deploy"},
		{"Deploy (prod), v2.0", "Deploy (prod), v2.0"},
		{"n/a", "n/a"},
		{"Deploy: staging", "'Deploy: staging'"}, // SQL: "Deploy: staging"
		{"Fix #12", "'Fix #12'"},                 // SQL: "Fix #12"
		{"yes", "yes"},                           // SQL: "yes"
	} {
		got := renderSkill(store.AgentSkill{Name: "d", Description: c.description, Triggers: []string{"user", "model"}, Body: "x\n"}, "")
		if want := "---\nname: d\ndescription: " + c.want + "\n---\nx\n"; got != want {
			t.Errorf("description %q rendered %q, want %q", c.description, got, want)
		}
	}
}

// A version row 0210 backfilled in SQL reads with SQL-style quoting; the
// startup pass re-renders it, so the next save's diff shows no description line
// (unrelated edits only), and a second pass changes nothing.
func TestBackfilledSkillVersionsShowNoDescriptionLineInTheNextDiff(t *testing.T) {
	for _, c := range []struct {
		description, sql string
		differs          bool
	}{
		{"How to deploy", "How to deploy", false},
		{"Deploy (prod), v2.0", "Deploy (prod), v2.0", false},
		{"n/a", "n/a", false},
		{"Deploy: staging", `"Deploy: staging"`, true},
		{"Fix #12", `"Fix #12"`, true},
		{"yes", `"yes"`, true},
	} {
		t.Run(c.description, func(t *testing.T) {
			s := newSkillVersionSurfaces(t)
			ctx := caller("admin")
			created, err := s.svc.CreateAgentSkill(ctx, "tea-a", AgentSkillInput{Name: str("d"), Description: str(c.description), Body: str("x\n")})
			if err != nil {
				t.Fatal(err)
			}
			// Version 1 as the 0210 SQL wrote it.
			fake := s.svc.Skills.(*fakeSkills)
			sqlContent := "---\nname: d\ndescription: " + c.sql + "\n---\nx\n"
			fake.versions[created.ID][0].Content = sqlContent
			fake.versions[created.ID][0].ShortID = store.AgentSkillVersionShortID(created.ID, 1, sqlContent)

			n, err := BackfillSkillVersionContent(context.Background(), fake)
			if want := map[bool]int{true: 1, false: 0}[c.differs]; err != nil || n != want {
				t.Fatalf("backfill rewrote %d rows (err %v), want %d", n, err, want)
			}
			v1 := fake.versions[created.ID][0]
			if v1.Content != created.Content || v1.ShortID != store.AgentSkillVersionShortID(created.ID, 1, created.Content) {
				t.Fatalf("version 1 = %q (%s), want the Go rendering %q", v1.Content, v1.ShortID, created.Content)
			}
			if again, err := BackfillSkillVersionContent(context.Background(), fake); err != nil || again != 0 {
				t.Fatalf("second backfill rewrote %d rows (err %v)", again, err)
			}

			if _, err := s.svc.UpdateAgentSkill(ctx, "tea-a", created.ID, AgentSkillInput{Body: str("y\n"), IfVersion: &created.Version}); err != nil {
				t.Fatal(err)
			}
			v2, err := s.svc.AgentSkillVersion(ctx, "tea-a", created.ID, 2)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(v2.Diff, "-x\n+y\n") || strings.Contains(v2.Diff, "\n-description") || strings.Contains(v2.Diff, "\n+description") {
				t.Fatalf("diff of the next save = %q, want only the body change", v2.Diff)
			}
		})
	}
}

// Rows that are not 0210 output are left alone: a Go rendering, a row whose
// content is not the SQL rendering of its own fields, and non-skill text.
func TestSkillVersionBackfillLeavesOtherRowsAlone(t *testing.T) {
	for _, content := range []string{
		"---\nname: d\ndescription: 'Fix #12'\n---\nx\n",               // already the Go rendering
		"---\nname: d\ndescription: \"Fix #12\"\nextra:   1\n---\nx\n", // hand-written spacing: not what 0210 emits
		"---\nname: d\ndescription: \"Fix #12\"\ntriggers: [user, model]\n---\nx\n",
		"not a skill",
	} {
		if got, ok := rerenderBackfilledSkillVersion(content); ok {
			t.Errorf("rewrote %q to %q", content, got)
		}
	}
	// A user-only skill with extra frontmatter, as 0210 writes it.
	sql := "---\nname: d\ndescription: \"Cut: a release\"\ntriggers: [user]\nargument-hint: <tag>\n---\nSteps.\n"
	want := "---\nname: d\ndescription: 'Cut: a release'\ntriggers: [user]\nargument-hint: <tag>\n---\nSteps.\n"
	if got, ok := rerenderBackfilledSkillVersion(sql); !ok || got != want {
		t.Errorf("user-only skill = %q %v, want %q", got, ok, want)
	}
}
