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

package github

import (
	"context"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// RuntimeDetection is the dashboard-facing verdict for one repository
// directory. Empty fields mean unknown; GraphQL renders them as null.
type RuntimeDetection struct {
	Runtime         string
	MatchedManifest string
}

type runtimeManifest struct {
	name    string
	runtime string
}

const dockerManifest = "Dockerfile"

// Render's public docs do not publish its auto-detection precedence. The
// milestone's conservative fallback is therefore Dockerfile-first, with native
// manifests in one tier. If more than one native runtime appears, returning
// unknown is safer than silently choosing an undocumented language precedence.
var nativeRuntimeManifests = []runtimeManifest{
	{name: "go.mod", runtime: "go"},
	{name: "package.json", runtime: "node"},
	{name: "requirements.txt", runtime: "python"},
	{name: "pyproject.toml", runtime: "python"},
	{name: "Gemfile", runtime: "ruby"},
	{name: "mix.exs", runtime: "elixir"},
	{name: "Cargo.toml", runtime: "rust"},
}

// DetectRuntime is a pure manifest-to-runtime mapping. Dockerfile wins; one
// unique native runtime wins; no signal or conflicting native signals are
// unknown. Multiple manifests for the same runtime (Python's two supported
// forms) are not ambiguous and retain the first table entry as evidence.
func DetectRuntime(entries []RepoTreeEntry) RuntimeDetection {
	files := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.Type == "file" {
			files[entry.Name] = struct{}{}
		}
	}
	if _, ok := files[dockerManifest]; ok {
		return RuntimeDetection{Runtime: "docker", MatchedManifest: dockerManifest}
	}
	var detection RuntimeDetection
	for _, manifest := range nativeRuntimeManifests {
		if _, ok := files[manifest.name]; !ok {
			continue
		}
		if detection.Runtime == "" {
			detection = RuntimeDetection{Runtime: manifest.runtime, MatchedManifest: manifest.name}
			continue
		}
		if detection.Runtime != manifest.runtime {
			return RuntimeDetection{}
		}
	}
	return detection
}

func (s *Service) runtimeDetectionMemo() *core.TTLCache[RuntimeDetection] {
	s.runtimeDetectionOnce.Do(func() {
		s.runtimeDetectionCache = core.NewTTLCache[RuntimeDetection]()
	})
	return s.runtimeDetectionCache
}

// DetectRepoRuntime combines the authorized GitHub probe with the pure
// heuristic. Probe failures and unrecognized trees intentionally return an
// empty verdict without a transport error so the wizard remains manual.
func (s *Service) DetectRepoRuntime(ctx context.Context, ownerID, repoURL, branch, rootDir string) (RuntimeDetection, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return RuntimeDetection{}, err
	}
	target, ok, err := s.repoTreeTarget(ctx, repoURL, branch, rootDir)
	if err != nil {
		return RuntimeDetection{}, err
	}
	if !ok {
		return RuntimeDetection{}, nil
	}
	cacheKey := target.cacheKey()
	if cached, ok := s.runtimeDetectionMemo().Get(cacheKey); ok {
		return cached, nil
	}
	value, err, _ := s.runtimeDetectionFlight.Do(cacheKey, func() (any, error) {
		if cached, ok := s.runtimeDetectionMemo().Get(cacheKey); ok {
			return cached, nil
		}
		probe, err := s.probeRepoTree(ctx, target)
		if err != nil {
			return RuntimeDetection{}, err
		}
		detection := RuntimeDetection{}
		ttl := runtimeDetectionUnknownTTL
		if !probe.Unknown {
			detection = DetectRuntime(probe.Entries)
			if detection.Runtime != "" {
				ttl = runtimeDetectionCacheTTL
			}
		}
		s.runtimeDetectionMemo().Put(cacheKey, detection, time.Now().Add(ttl))
		return detection, nil
	})
	if err != nil {
		return RuntimeDetection{}, err
	}
	return value.(RuntimeDetection), nil
}
