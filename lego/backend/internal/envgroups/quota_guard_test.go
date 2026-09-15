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

package envgroups

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
)

// The environment group's batch patch is w1/m147's control: its three surfaces
// already refused an over-quota secret file with the same sentence the service
// batch patch now uses (t005's env-group rows).
func TestEnvGroupPatchQuotaRefusalIsIdenticalAcrossAdapters(t *testing.T) {
	const want = "total secret file size limit of 524288 bytes exceeded"
	big := strings.Repeat("x", 614_400)
	ctx := context.Background()

	fixture := func(t *testing.T) (*Service, EnvGroupView) {
		t.Helper()
		svc := newService(newFakeStore(), sampleApp("web"))
		group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
		if err != nil {
			t.Fatalf("fixture group: %v", err)
		}
		return svc, group
	}
	stored := func(t *testing.T, svc *Service, group EnvGroupView) {
		t.Helper()
		if _, err := svc.GetEnvGroupFile(ctx, group.ID, "big.bin"); err == nil {
			t.Fatal("the refusal stored the file")
		}
	}

	t.Run("REST", func(t *testing.T) {
		svc, group := fixture(t)
		// The group batch patch is /contents; /environment moves the group.
		response := serveREST(svc, "PATCH", "/v1/env-groups/"+group.ID+"/contents",
			`{"saveMode":"save_only","secretFiles":[{"name":"big.bin","content":"`+big+`"}]}`)
		if response.Code != 400 || !strings.Contains(response.Body.String(), want) {
			t.Fatalf("REST = %d %s, want 400 naming the quota", response.Code, response.Body.String())
		}
		stored(t, svc, group)
	})

	t.Run("GraphQL", func(t *testing.T) {
		svc, group := fixture(t)
		_, err := svc.GraphQLMutation()["patchEnvGroupEnvironment"].Resolve(graphql.ResolveParams{Context: ctx, Args: map[string]any{
			"id": group.ID, "saveMode": "save_only",
			"secretFiles": []any{map[string]any{"name": "big.bin", "content": big}},
		}})
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), want) {
			t.Fatalf("GraphQL = %v, want a bad request naming the quota", err)
		}
		stored(t, svc, group)
	})

	t.Run("MCP", func(t *testing.T) {
		svc, group := fixture(t)
		srv := mcp.NewServer(&mcp.Implementation{Name: "bex", Version: "0"}, nil)
		svc.RegisterMCP(srv)
		serverTransport, clientTransport := mcp.NewInMemoryTransports()
		if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
			t.Fatalf("server connect: %v", err)
		}
		cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
		if err != nil {
			t.Fatalf("client connect: %v", err)
		}
		t.Cleanup(func() { _ = cs.Close() })

		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "patch_env_group_environment", Arguments: map[string]any{
			"id": group.ID, "saveMode": "save_only",
			"secretFiles": []map[string]any{{"name": "big.bin", "content": big}},
		}})
		encoded, _ := json.Marshal(res)
		if err == nil && (res == nil || !res.IsError) {
			t.Fatalf("MCP accepted the over-quota patch: %s", encoded)
		}
		if !strings.Contains(string(encoded), want) && (err == nil || !strings.Contains(err.Error(), want)) {
			t.Fatalf("MCP refusal lost the quota sentence: err=%v result=%s", err, encoded)
		}
		stored(t, svc, group)
	})
}

// TestEveryEnvGroupMapWriterRefusesAnOverQuotaWrite keeps the environment
// group's own writes guarded — the control w1/m147 compared the service batch
// patch against — the same way secrets' TestEveryServiceMapWriterRefusesAnOverQuotaWrite
// guards a service's. Behavioural: each exported writer gets an over-quota
// input and must refuse it with core.ErrBadRequest, leaving the group's maps as
// they were; a reflection sweep fails when a writer-named verb has neither a
// case nor a reasoned exemption.
func TestEveryEnvGroupMapWriterRefusesAnOverQuotaWrite(t *testing.T) {
	big := strings.Repeat("x", 512*1024+1)
	ctx := context.Background()

	writers := map[string]func(*Service, EnvGroupView) error{
		"CreateEnvGroup": func(s *Service, _ EnvGroupView) error {
			_, err := s.CreateEnvGroup(ctx, CreateEnvGroupRequest{
				Name:    "too-big",
				EnvVars: []CreateEnvVarInput{{Key: "BIG", Value: big}},
			})
			return err
		},
		"SetEnvGroupVars": func(s *Service, g EnvGroupView) error {
			_, err := s.SetEnvGroupVars(ctx, g.ID, []EnvVarView{{Key: "BIG", Value: big}})
			return err
		},
		"SetEnvGroupVar": func(s *Service, g EnvGroupView) error {
			_, err := s.SetEnvGroupVar(ctx, g.ID, "BIG", big)
			return err
		},
		"SetEnvGroupVarInput": func(s *Service, g EnvGroupView) error {
			_, err := s.SetEnvGroupVarInput(ctx, g.ID, EnvVarView{Key: "BIG", Value: big})
			return err
		},
		"SetEnvGroupFile": func(s *Service, g EnvGroupView) error {
			_, err := s.SetEnvGroupFile(ctx, g.ID, "big.bin", big)
			return err
		},
		"PatchEnvironment": func(s *Service, g EnvGroupView) error {
			_, err := s.PatchEnvironment(ctx, g.ID, EnvironmentPatch{
				ExpectedRevision: &g.Revision,
				SaveMode:         SaveModeOnly,
				SecretFiles:      []SecretFilePatch{{Name: "big.bin", Content: big}},
			})
			return err
		},
		"ApplyEnvGroup": func(s *Service, g EnvGroupView) error {
			return s.ApplyEnvGroup(ctx, g.Name, map[string]string{"BIG": big}, nil)
		},
	}
	exempt := map[string]string{
		"SetEnvironmentID":    "moves the group between environments; writes no env or file map",
		"SetGroupEnvironment": "moves the group between environments; writes no env or file map",
		"MoveEnvGroup":        "moves the group between environments; writes no env or file map",
		"CloneEnvGroup":       "copies a source group whose maps were quota-checked when they were written",
	}

	for name, write := range writers {
		t.Run(name, func(t *testing.T) {
			svc := newService(newFakeStore(), sampleApp("web"))
			group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
				Name:        "shared",
				EnvVars:     []CreateEnvVarInput{{Key: "KEEP", Value: "small"}},
				SecretFiles: []SecretFileView{{Name: "keep.pem", Content: "small"}},
			})
			if err != nil {
				t.Fatalf("fixture group: %v", err)
			}
			if err := write(svc, group); !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("%s with an over-quota input = %v, want core.ErrBadRequest", name, err)
			}
			for key, want := range map[string]string{"KEEP": "small"} {
				got, err := svc.GetEnvGroupVar(ctx, group.ID, key)
				if err != nil || got.Value != want {
					t.Fatalf("%s changed the group's env map: %s = %+v, %v", name, key, got, err)
				}
			}
			if _, err := svc.GetEnvGroupVar(ctx, group.ID, "BIG"); err == nil {
				t.Fatalf("%s stored the over-quota variable", name)
			}
			if _, err := svc.GetEnvGroupFile(ctx, group.ID, "big.bin"); err == nil {
				t.Fatalf("%s stored the over-quota file", name)
			}
		})
	}

	writerName := regexp.MustCompile(`^(Set|Seed|Patch|Replace|Create|Clone|Import|Apply|Add|Upsert|Put|Store|Write|Copy|Move|Merge)`)
	base := reflect.TypeOf(&core.Base{})
	svcType := reflect.TypeOf(&Service{})
	for i := 0; i < svcType.NumMethod(); i++ {
		method := svcType.Method(i).Name
		if _, promoted := base.MethodByName(method); promoted || !writerName.MatchString(method) {
			continue
		}
		if _, ok := writers[method]; ok {
			continue
		}
		if _, ok := exempt[method]; !ok {
			t.Errorf("exported writer %s has neither an over-quota case nor a reasoned exemption", method)
		}
	}
	_ = secrets.ValidateEnvMapQuota // the quota these writers share
}
