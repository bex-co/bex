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
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// docker_context.go is the one seam between Render's dockerContext and the
// stored spec.dockerContext (w8/m128). On the wire, as on Render, the context
// is relative to rootDir and defaults to "." (Render's monorepo guide: "All of
// the following settings operate relative to the root directory … Docker build
// context directory"). The App CR keeps the repository-root path the build
// reads, empty meaning rootDir itself, so no stored App and no build changed
// when the wire meaning did.

// storedDockerContext maps a rootDir-relative wire value onto the stored
// repository-root path. A context equal to rootDir stores empty, so
// {rootDir: backend, dockerContext: "."} and {rootDir: backend} are the same
// spec. Callers validate the wire value first (no "..", no absolute path).
func storedDockerContext(rootDir, wire string) string {
	wire = strings.TrimSpace(wire)
	if wire == "" {
		return ""
	}
	joined := path.Join(rootDir, wire)
	if joined == path.Clean(cmpRoot(rootDir)) {
		return ""
	}
	return joined
}

// wireDockerContext reports a stored context relative to rootDir: "." for the
// default (empty, or equal to rootDir). A context stored outside rootDir — one
// written before w8/m128 under the repository-root reading — reports its path
// relative to rootDir, "../…" included, rather than a value that would point
// somewhere else.
func wireDockerContext(rootDir, stored string) string {
	if stored == "" {
		return "."
	}
	rel, err := filepath.Rel(cmpRoot(rootDir), stored)
	if err != nil {
		return "." // unreachable: both are clean relative paths
	}
	return filepath.ToSlash(rel)
}

// rebaseDockerContext keeps a stored context's rootDir-relative meaning across
// a rootDir change, as Render's settings move with the root directory. An
// empty context stays empty; one outside the old rootDir is left as stored.
func rebaseDockerContext(oldRoot, newRoot, stored string) string {
	if stored == "" {
		return ""
	}
	rel := wireDockerContext(oldRoot, stored)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return stored
	}
	return storedDockerContext(newRoot, rel)
}

// cmpRoot is rootDir as a path: the repository root is ".".
func cmpRoot(rootDir string) string {
	if rootDir == "" {
		return "."
	}
	return rootDir
}

// viewDockerContext is AppView's dockerContext: the wire value for a Dockerfile
// build ("." when unset, as Render reports it), and for any other build only a
// context it still stores from before create refused one (w5/090).
func viewDockerContext(spec appv1alpha1.AppSpec) string {
	if spec.DockerContext == "" && buildStrategy(spec) != buildDockerfile {
		return ""
	}
	return wireDockerContext(spec.RootDir, spec.DockerContext)
}

// repoRootRelative reads a Blueprint path that starts at the repository root,
// as Render's templates write it ("/" for the root itself, "/docker" for a
// directory), as the relative path bex stores: "." and "docker" (w5/172).
// Any other path, and a ".." component, is left for the path rules to judge.
func repoRootRelative(p string) string {
	if !strings.HasPrefix(p, "/") {
		return p
	}
	if p = strings.TrimLeft(p, "/"); p == "" {
		return "."
	}
	return p
}

// checkDockerContextPath is create's and update's one rule for the wire value:
// a relative path with no ".." component.
func checkDockerContextPath(wire string) error {
	if wire != "" && !store.ValidRootDir(wire) {
		return fmt.Errorf("%w: dockerContext must be a relative path with no '..' components", core.ErrBadRequest)
	}
	return nil
}
