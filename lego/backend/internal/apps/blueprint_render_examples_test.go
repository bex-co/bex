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

package apps

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// w1/m175: Render's own examples copy a private service's generated password
// into the service that uses it. Each testdata/render-<name>/render.yaml is the
// upstream file byte for byte:
//
//	render-examples/ghost      8c353f7a59761f704dc2272b0997e4975786e5b8
//	render-examples/wordpress  94c6a05f0f00d8806d5b291ba5620a1f1f1ab653
//	render-examples/mattermost 51a1d9ae5321f873546f95ec4a701a49cb977414
//
// Running the MySQL and MinIO images is ADR089's concern, not this one.
func TestRenderExampleBlueprintsCopyTheirGeneratedPasswords(t *testing.T) {
	for _, tc := range []struct {
		example  string
		consumer string
		want     []generatedCopy
		// refused is the one unrelated refusal the unchanged file still meets,
		// and rename makes a copy of the file that avoids it, so the copies are
		// checked anyway.
		refused string
		rename  func(string) string
	}{
		{"ghost", "ghost", []generatedCopy{
			{key: "database__connection__password", source: "mysql", sourceKey: "MYSQL_PASSWORD"},
		}, "", nil},
		{"wordpress", "wordpress", []generatedCopy{
			{key: "WORDPRESS_DB_PASSWORD", source: "mysql-wordpress", sourceKey: "MYSQL_PASSWORD"},
		}, "", nil},
		// mattermost names its database and its web service alike, which bex
		// refuses and Render accepts (w1/126); that is not the copy.
		{"mattermost", "mattermost", []generatedCopy{
			{key: "MM_FILESETTINGS_AMAZONS3ACCESSKEYID", source: "mattermost-minio", sourceKey: "MINIO_ACCESS_KEY"},
			{key: "MM_FILESETTINGS_AMAZONS3SECRETACCESSKEY", source: "mattermost-minio", sourceKey: "MINIO_SECRET_KEY"},
		}, `duplicate name "mattermost"`, func(m string) string {
			m = strings.Replace(m, "- name: mattermost\n  plan: basic-256mb", "- name: mattermost-db\n  plan: basic-256mb", 1)
			return strings.ReplaceAll(m, "fromDatabase:\n      name: mattermost\n", "fromDatabase:\n      name: mattermost-db\n")
		}},
	} {
		t.Run(tc.example, func(t *testing.T) {
			manifest, err := os.ReadFile("testdata/render-" + tc.example + "/render.yaml")
			if err != nil {
				t.Fatal(err)
			}
			repo := "https://github.com/render-examples/" + tc.example
			stack, err := parseStack(DeployRequest{Manifest: string(manifest), Repo: repo, Branch: "master"})
			if tc.refused != "" {
				if err == nil || !strings.Contains(err.Error(), tc.refused) {
					t.Fatalf("unchanged file: err = %v, want only the unrelated %q refusal", err, tc.refused)
				}
				stack, err = parseStack(DeployRequest{Manifest: tc.rename(string(manifest)), Repo: repo, Branch: "master"})
			}
			if err != nil {
				t.Fatalf("the upstream Blueprint was refused: %v", err)
			}
			var copies []generatedCopy
			for _, svc := range stack.services {
				if svc.req.Name == tc.consumer {
					copies = svc.generatedCopies
				}
				for _, e := range svc.req.Env {
					for _, c := range tc.want {
						if svc.req.Name == tc.consumer && e.Name == c.key {
							t.Fatalf("%s reached %s's spec.Env as %+v: a secret must never ride the App spec", c.key, tc.consumer, e)
						}
					}
				}
			}
			for _, c := range tc.want {
				if !slices.Contains(copies, c) {
					t.Fatalf("%s copies = %+v, missing %+v", tc.consumer, copies, c)
				}
			}
		})
	}
}
