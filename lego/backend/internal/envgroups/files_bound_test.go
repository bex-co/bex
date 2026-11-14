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
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// filesOverQuota is the refusal past a group's secret-files bound (w4/m363).
const filesOverQuota = "the combined size of all secret files cannot exceed 1 MB (1048576 bytes)"

// w4/m363: a group's secret files hold Render's 1 MB, counted on contents.
// Each group files writer takes exactly 1 MiB of contents under two maximal
// names, and refuses one byte more.
func TestEveryEnvGroupFilesWriterTakesExactlyTheContentsBound(t *testing.T) {
	ctx := testCtx()
	nameA, nameB := strings.Repeat("a", 249)+".txt", strings.Repeat("b", 249)+".txt"
	files := func(total int) []SecretFileView {
		return []SecretFileView{
			{Name: nameA, Content: strings.Repeat("c", 614_400)},
			{Name: nameB, Content: strings.Repeat("d", total-614_400)},
		}
	}
	emptyGroup := func(s *Service) (EnvGroupView, error) {
		return s.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "certs"})
	}
	writers := map[string]func(*Service, []SecretFileView) error{
		"CreateEnvGroup": func(s *Service, in []SecretFileView) error {
			_, err := s.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "certs", SecretFiles: in})
			return err
		},
		"SetEnvGroupFile": func(s *Service, in []SecretFileView) error {
			g, err := emptyGroup(s)
			if err != nil {
				return err
			}
			for _, f := range in {
				if _, err := s.SetEnvGroupFile(ctx, g.ID, f.Name, f.Content); err != nil {
					return err
				}
			}
			return nil
		},
		"PatchEnvironment": func(s *Service, in []SecretFileView) error {
			g, err := emptyGroup(s)
			if err != nil {
				return err
			}
			patch := EnvironmentPatch{ExpectedRevision: &g.Revision, SaveMode: SaveModeOnly}
			for _, f := range in {
				patch.SecretFiles = append(patch.SecretFiles, SecretFilePatch{Name: f.Name, Content: f.Content})
			}
			_, err = s.PatchEnvironment(ctx, g.ID, patch)
			return err
		},
	}
	for name, write := range writers {
		t.Run(name+"/at the bound", func(t *testing.T) {
			if err := write(newService(newFakeStore(), sampleApp("web")), files(1<<20)); err != nil {
				t.Fatalf("a group map of exactly 1 MiB of contents = %v, want accepted", err)
			}
		})
		t.Run(name+"/one byte over", func(t *testing.T) {
			err := write(newService(newFakeStore(), sampleApp("web")), files(1<<20+1))
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), filesOverQuota) {
				t.Fatalf("one byte past the bound = %v, want a bad request naming %q", err, filesOverQuota)
			}
		})
	}
}
