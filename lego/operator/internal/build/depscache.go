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

import "strings"

// A native runtime's dependency directories across builds (w9/m494,
// docs/ADR060 D3).
//
// Render keeps the directories a native runtime installs its dependencies
// into from one build of a service to the next ("Render already caches well
// known paths for all native language environments"), so a source-only
// commit installs nothing again: Bundler finds its gems, Go its modules,
// Cargo its registry and compiled dependencies, Mix its deps and their
// build. "Clear build cache & deploy" empties them with the rest of the cache.
//
// Unlike $XDG_CACHE_HOME, these directories are part of the service: Bundler
// loads its gems from BUNDLE_PATH at runtime, `cargo run` and `mix phx.server`
// read target/ and _build/. So they cannot be a cache mount, which would keep
// them out of the image, and a mountpoint the build command cannot remove
// (`cargo clean`, `rm -rf _build`) would fail builds Render runs. They stay
// ordinary directories of the image. The build command's RUN also mounts a
// cache mount at nativeDepsCacheDir: before the command, the env loader moves
// the previous build's copy from it into each directory the source did not
// bring; after the command, a step of its own copies the directories back.
// The buildkit phase's seed and export solves (xdgcache.go) carry the mount
// in the same archive as $XDG_CACHE_HOME, Render's one build cache, and keep
// it only for the toolchain that built it.
const (
	// nativeDepsCacheDir is where the build command's RUN and the step after
	// it mount the dependency cache. Each directory sits under it at its own
	// absolute path.
	nativeDepsCacheDir = "/opt/bex/deps-cache"
	// nativeDepsCacheID names that cache mount across the build pod's solves,
	// as nativeXDGCacheID does $XDG_CACHE_HOME's.
	nativeDepsCacheID = "bex-deps-cache"
	// nativeDepsCacheMount is the build command's mount of the cache.
	nativeDepsCacheMount = " --mount=type=cache,id=" + nativeDepsCacheID + ",target=" + nativeDepsCacheDir + ",sharing=locked"
	// nativeDepsSaveEOF closes the save step's heredoc.
	nativeDepsSaveEOF = "BEX_DEPS_SAVE"
)

// nativeDependencyDirs is each native runtime's dependency directories, at
// Render's paths (render.com/docs/environment-variables):
//
//   - Ruby: the gems Bundler installs under BUNDLE_PATH,
//     /opt/render/project/.gems (a path Render caches by its own account), in
//     its ruby/<ABI version> layout, compiled extensions included.
//   - Go: the module cache and checksum database under GOPATH,
//     /opt/render/project/go/pkg. Go's build cache is under $XDG_CACHE_HOME
//     already.
//   - Rust: Cargo's registry and git checkouts under CARGO_HOME,
//     /opt/render/project/.cargo (a link to the toolchain's own, whose bin
//     holds the toolchain and is the image's), and the target directory at
//     the service root, which Render caches ("We're caching any target
//     directory at your repo root").
//   - Elixir: Mix's deps and _build at the service root, kept together:
//     _build links into deps, and a compiled dependency's NIFs are written
//     into deps.
//
// What a build regenerates and a cold build starts without stays out:
// .gems/bin and go/bin are first on PATH, so a binstub or tool kept there
// would run ahead of the toolchain's own before the build command has run
// (a kept `bundle` binstub names a Bundler RubyGems cannot activate from
// .gems, w9/128), and .gems/config holds `bundle config` settings a later
// build command may no longer make.
//
// Node's node_modules is w8/m161's. Python's virtualenv is not here yet:
// unlike these, the recipe makes it before the build (w9/127).
var nativeDependencyDirs = map[string][]string{
	nativeRubyRuntime:   {nativeRubyGems + "/ruby"},
	"go":                {nativeGoPath + "/pkg"},
	nativeRustRuntime:   {nativeCargoHome + "/registry", nativeCargoHome + "/git", nativeWorkdir + "/target"},
	nativeElixirRuntime: {nativeWorkdir + "/deps", nativeWorkdir + "/_build"},
}

// depsCache is the dependency directories this build carries to the next
// one: those of a native build that keeps $XDG_CACHE_HOME in the S3 store.
// Anywhere else the cache mount would start empty and end discarded, so the
// copies in and out would cost the build and save nothing.
func (o Options) depsCache() []string {
	if !o.xdgCache() {
		return nil
	}
	return nativeDependencyDirs[nativeRuntime(o)]
}

// nativeDepsRecipe is what the recipe adds for a build that carries dirs:
// the build command's mount of the cache, the env loader's seed and the save
// step after the command. A build that carries none gets none of them.
func nativeDepsRecipe(dirs []string) (mount, seed, save string) {
	if len(dirs) == 0 {
		return "", "", ""
	}
	return nativeDepsCacheMount, nativeDepsSeed(dirs), nativeDepsSave(dirs)
}

// nativeDepsWords is dirs as shell words. Every path is a constant of this
// package, so quoting is belt and braces.
func nativeDepsWords(dirs []string) string {
	words := make([]string, len(dirs))
	for i, dir := range dirs {
		words[i] = shellSingleQuote(dir)
	}
	return strings.Join(words, " ")
}

// nativeDepsSeed is the env loader's first part when the build carries
// dependency directories: it moves each one the seed solve restored into
// place, before the service's variables are exported, so a variable of
// theirs (PATH) cannot change what it runs.
//
// A directory the source brought, or the toolchain image made, is the
// build's: only one that is missing or empty is replaced. Each arrives whole
// or not at all: it is moved beside its place, then renamed into it. What is
// left in the mount is deleted, so the build has its disk. The move is quiet,
// and never fails the build: the command then installs from nothing, as a
// first build does.
func nativeDepsSeed(dirs []string) string {
	return `bex_deps_seed() {
  for bex_dep in "$@"; do
    [ -d "` + nativeDepsCacheDir + `$bex_dep" ] || continue
    if [ -e "$bex_dep" ] || [ -L "$bex_dep" ]; then
      [ -d "$bex_dep" ] && [ ! -L "$bex_dep" ] && rmdir "$bex_dep" || continue
    fi
    rm -rf "$bex_dep.bex-seed"
    mkdir -p "${bex_dep%/*}" && mv "` + nativeDepsCacheDir + `$bex_dep" "$bex_dep.bex-seed" && mv "$bex_dep.bex-seed" "$bex_dep" || rm -rf "$bex_dep.bex-seed"
  done
  find ` + nativeDepsCacheDir + ` -mindepth 1 -delete
} >/dev/null 2>&1
bex_deps_seed ` + nativeDepsWords(dirs) + `
unset -f bex_deps_seed
unset bex_dep
`
}

// nativeDepsSave is the step after the build command's when the build
// carries its dependency directories: it copies each directory the command
// left into the cache mount, for the export solve to archive. Only a
// directory, never a link, is copied, and links inside one stay links. A
// total over the archive's bound copies nothing, since the archive could not
// keep it; a copy that fails leaves nothing of itself. The step writes only
// into the mount, so its layer is empty, and it never fails the build. A
// build served from the layer cache runs neither step, and the mount keeps
// what the seed solve restored.
func nativeDepsSave(dirs []string) string {
	return "RUN --network=none" + nativeDepsCacheMount + " <<'" + nativeDepsSaveEOF + `'
{
find ` + nativeDepsCacheDir + ` -mindepth 1 -delete
set --
for bex_dep in ` + nativeDepsWords(dirs) + `; do
  [ -d "$bex_dep" ] && [ ! -L "$bex_dep" ] && set -- "$@" "$bex_dep"
done
[ "$#" -gt 0 ] || exit 0
bex_bytes=$(du -sbc "$@" | tail -n 1 | cut -f1)
[ "$bex_bytes" -le ` + xdgMaxBytes + ` ] || exit 0
for bex_dep in "$@"; do
  mkdir -p "` + nativeDepsCacheDir + `${bex_dep%/*}" && cp -a "$bex_dep" "` + nativeDepsCacheDir + `$bex_dep" || rm -rf "` + nativeDepsCacheDir + `$bex_dep"
done
} >/dev/null 2>&1
exit 0
` + nativeDepsSaveEOF + "\n"
}
