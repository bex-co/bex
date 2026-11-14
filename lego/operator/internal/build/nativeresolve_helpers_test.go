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

package build

import (
	"context"
	"time"
)

// resolveNativeBuild runs the resolve phase with no service creation date and
// the live Docker Hub resolver, the shape most native-version fixtures use.
func resolveNativeBuild(runtime, bundlePath, dockerfilePath, sourceDir string) ([]NativeChoice, error) {
	return resolveNativeBuildAt(time.Time{}, runtime, bundlePath, dockerfilePath, sourceDir)
}

// resolveNativeBuildAt is resolveNativeBuild for a service created at created,
// whose creation date picks the default Python line (w8/174) and uv's default
// (w8/328).
func resolveNativeBuildAt(created time.Time, runtime, bundlePath, dockerfilePath, sourceDir string) ([]NativeChoice, error) {
	return ResolveNativeBuildExact(context.Background(), NewDockerHubResolver(), created, runtime, bundlePath, dockerfilePath, sourceDir)
}
