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
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// A primary Node service image starts its command under bex's native-init
// (w8/m88), so an app behind `npm start` still gets SIGTERM and finishes its
// in-flight work when npm exits first. The binary ships in bex's own image:
// the resolve phase, which runs that image, stages it for BuildKit as a named
// context, so the build pulls nothing for it.
const (
	// NativeInitBinary is native-init's path in the bex image.
	NativeInitBinary = "/" + nativeInitFile
	// NativeInitEnv carries the staging path to the resolve phase.
	NativeInitEnv = "BEX_NATIVE_INIT"

	nativeInitFile    = "native-init"
	nativeInitDir     = "/native/init"
	nativeInitContext = "bex-init"
	// nativeInitPath is where the image carries it: the root, because a
	// COPY --link that creates a parent directory is not byte-reproducible
	// (nativeMediaTools).
	nativeInitPath = "/" + nativeInitFile
)

// nativeServingInit reports whether o's image starts under native-init: a
// Node service whose build has the resolve phase to stage it.
func nativeServingInit(o Options) bool {
	return !o.StaticSite && o.NativeResolverImage != "" && nativeRuntime(o) == nativeNodeRuntime
}

// nativeEntrypoint replaces the toolchain image's ENTRYPOINT, whose relative
// lookup an empty PATH would break (w8/m61), with nothing or with
// native-init's absolute path; the CMD after it stays the exact command.
func nativeEntrypoint(o Options) string {
	if !nativeServingInit(o) {
		return "ENTRYPOINT []\n"
	}
	return fmt.Sprintf("COPY --link --from=%s /%s /\nENTRYPOINT [%q]\n",
		nativeInitContext, nativeInitFile, nativeInitPath)
}

// StageNativeInit copies native-init from src to dest, in the directory the
// build receives as its named context. dest keeps src's mtime: COPY keeps the
// file's, and the release's own keeps the layer identical across builds.
func StageNativeInit(src, dest string) error {
	if err := stageNativeInit(src, dest); err != nil {
		return fmt.Errorf("stage native-init: %w", err)
	}
	return nil
}

func stageNativeInit(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dest, info.ModTime(), info.ModTime())
}
