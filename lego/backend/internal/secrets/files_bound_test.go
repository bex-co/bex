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

package secrets

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// longName is a 253-character secret-file name, the longest a Secret key takes.
func longName(c byte) string { return strings.Repeat(string(c), 249) + ".txt" }

// w4/m363: a service's secret files hold Render's "combined size … cannot
// exceed 1 MB", counted on contents. Every files writer accepts a map whose
// contents total exactly the bound, however long its names, and refuses one
// byte more with the sentence that names the limit.
func TestEveryFilesWriterTakesExactlyTheContentsBound(t *testing.T) {
	ctx := context.Background()
	if maxSecretFilesBytes != 1<<20 {
		t.Fatalf("maxSecretFilesBytes = %d, want 1 MiB (Render's 1 MB, at most Kubernetes' MaxSecretSize)", maxSecretFilesBytes)
	}
	// Two maximal names and the 614,400-byte file run sf13 was refused: names
	// would push a names-plus-contents count past the bound, contents alone do not.
	first, second := longName('a'), longName('b')
	split := func(total int) []SecretFileView {
		return []SecretFileView{
			{Name: first, Content: strings.Repeat("c", 614_400)},
			{Name: second, Content: strings.Repeat("d", total-614_400)},
		}
	}
	writers := map[string]func(*Service, []SecretFileView) error{
		"SetSecretFile": func(s *Service, files []SecretFileView) error {
			for _, f := range files {
				if _, err := s.SetSecretFile(ctx, "web", f.Name, f.Content); err != nil {
					return err
				}
			}
			return nil
		},
		"SetSecretFiles": func(s *Service, files []SecretFileView) error {
			_, err := s.SetSecretFiles(ctx, "web", files)
			return err
		},
		"SeedSecretFiles": func(s *Service, files []SecretFileView) error {
			seed := make([]core.SecretFile, 0, len(files))
			for _, f := range files {
				seed = append(seed, core.SecretFile{Name: f.Name, Content: f.Content})
			}
			return s.SeedSecretFiles(ctx, "web", seed)
		},
		"CheckCreateSecrets": func(s *Service, files []SecretFileView) error {
			seed := make([]core.SecretFile, 0, len(files))
			for _, f := range files {
				seed = append(seed, core.SecretFile{Name: f.Name, Content: f.Content})
			}
			return createSecretsSeeder{service: s}.CheckCreateSecrets(seed, nil)
		},
		"PatchEnvironment": func(s *Service, files []SecretFileView) error {
			patch := EnvironmentPatch{SaveMode: SaveModeDeploy}
			for _, f := range files {
				patch.SecretFiles = append(patch.SecretFiles, SecretFilePatch{Name: f.Name, Content: f.Content})
			}
			_, err := s.PatchEnvironment(ctx, "web", patch)
			return err
		},
	}
	for name, write := range writers {
		t.Run(name+"/at the bound", func(t *testing.T) {
			store := newVersionedFakeSecretStore()
			if err := write(newService(store, sampleApp("web")), split(maxSecretFilesBytes)); err != nil {
				t.Fatalf("a map of exactly %d content bytes = %v, want accepted", maxSecretFilesBytes, err)
			}
			if name != "CheckCreateSecrets" && contentBytes(store.m[filesPath("web")]) != maxSecretFilesBytes {
				t.Fatalf("stored %d content bytes, want %d", contentBytes(store.m[filesPath("web")]), maxSecretFilesBytes)
			}
		})
		t.Run(name+"/one byte over", func(t *testing.T) {
			store := newVersionedFakeSecretStore()
			err := write(newService(store, sampleApp("web")), split(maxSecretFilesBytes+1))
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), filesOverQuota) {
				t.Fatalf("one byte past the bound = %v, want a bad request naming %q", err, filesOverQuota)
			}
			if contentBytes(store.m[filesPath("web")]) > maxSecretFilesBytes {
				t.Fatal("a refused write stored an over-bound map")
			}
		})
	}
}

// The env-var cap is not this milestone's: 600 KiB of variables is still
// refused with the unchanged sentence, and names still count toward it.
func TestEnvVarCapStaysAt512KiBOfKeysAndValues(t *testing.T) {
	ctx := context.Background()
	store := newVersionedFakeSecretStore()
	svc := newService(store, sampleApp("web"))
	vars := make([]EnvVarView, 0, 5)
	for _, key := range []string{"BIG_A", "BIG_B", "BIG_C", "BIG_D", "BIG_E"} {
		vars = append(vars, EnvVarView{Key: key, Value: strings.Repeat("x", 600<<10/5)})
	}
	_, err := svc.SetEnvVars(ctx, "web", vars)
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "total environment variable size limit of 524288 bytes exceeded") {
		t.Fatalf("600 KiB of env vars = %v, want the unchanged 512 KiB refusal", err)
	}
	if len(store.m[envPath("web")]) != 0 {
		t.Fatal("a refused env write reached the store")
	}
}

// An over-quota files map may shrink but not grow, measured as the quota is:
// renaming a file to a longer name adds no contents, so it stays possible.
func TestFilesPatchShrinkRuleCountsContents(t *testing.T) {
	over := map[string]string{"a": strings.Repeat("x", maxSecretFilesBytes+10)}
	renamed := map[string]string{longName('a'): over["a"]}
	if err := ValidatePatchFilesMapQuota(over, renamed); err != nil {
		t.Fatalf("renaming a file of an over-quota map = %v, want allowed (no contents added)", err)
	}
	grown := map[string]string{"a": over["a"] + "y"}
	if err := ValidatePatchFilesMapQuota(over, grown); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("growing an over-quota map = %v, want refused", err)
	}
}
