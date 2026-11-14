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

package store

import (
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/validationcorpus"
)

// The corpus files under testdata/validation pin each rule the dashboard's
// forms mirror; see core/validation_corpus_test.go.

func problemErr(problem string) error {
	if problem == "" {
		return nil
	}
	return errors.New(problem)
}

// TestRootDirCorpus pins the Root Directory and the Docker Build Context, which
// are judged after normalization (apps.checkDockerContextPath maps the spellings
// of one directory together first, as a Root Directory is).
func TestRootDirCorpus(t *testing.T) {
	t.Parallel()
	validationcorpus.Check(t, "root-dir", func(c validationcorpus.Case) error {
		return problemErr(RootDirProblem(NormalizeRootDir(validationcorpus.FormTrim(c.Text()))))
	})
}

// TestBuildPathCorpus pins the Dockerfile Path, judged as typed: it names a
// file, so an empty segment or a trailing slash is refused where the Root
// Directory drops it.
func TestBuildPathCorpus(t *testing.T) {
	t.Parallel()
	validationcorpus.Check(t, "build-path", func(c validationcorpus.Case) error {
		return problemErr(RootDirProblem(validationcorpus.FormTrim(c.Text())))
	})
}

// TestRepoURLCorpus pins a clone URL.
func TestRepoURLCorpus(t *testing.T) {
	t.Parallel()
	validationcorpus.Check(t, "repo-url", func(c validationcorpus.Case) error {
		if !ValidRepo(validationcorpus.FormTrim(c.Text())) {
			return errors.New("not a valid repository URL")
		}
		return nil
	})
}

// TestDockerImageCorpus pins an image reference under the default registry
// policy, any public registry, as on Render. It sets a package global, so it
// does not run in parallel.
func TestDockerImageCorpus(t *testing.T) {
	prev := PublicImageRegistries
	PublicImageRegistries = true
	t.Cleanup(func() { PublicImageRegistries = prev })
	validationcorpus.Check(t, "docker-image", func(c validationcorpus.Case) error {
		return ValidateImage(validationcorpus.FormTrim(c.Text()))
	})
}
