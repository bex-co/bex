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
	"os"
	"os/exec"
	"strings"
	"testing"
)

// staticDepsPackage is an npm static site whose build script runs a locked
// devDependency, as Eleventy's base blog runs cross-env (sweep358).
const staticDepsPackage = `{
  "name": "bex-static-deps-probe",
  "version": "1.0.0",
  "private": true,
  "scripts": {
    "build": "cross-env BEX_PROBE=built node build.js"
  },
  "devDependencies": {
    "cross-env": "7.0.3"
  }
}
`

// staticDepsBuild is the build script cross-env runs.
const staticDepsBuild = `require("fs").mkdirSync("_site", {recursive: true});
require("fs").writeFileSync("_site/index.html", "<p>" + process.env.BEX_PROBE + "</p>");
`

const staticDepsLock = `{
  "name": "bex-static-deps-probe",
  "version": "1.0.0",
  "lockfileVersion": 3,
  "requires": true,
  "packages": {
    "": {
      "name": "bex-static-deps-probe",
      "version": "1.0.0",
      "devDependencies": {
        "cross-env": "7.0.3"
      }
    },
    "node_modules/cross-env": {
      "version": "7.0.3",
      "resolved": "https://registry.npmjs.org/cross-env/-/cross-env-7.0.3.tgz",
      "integrity": "sha512-+/HKd6EgcQCJGh2PSjZuUitQBQynKor4wrFbRg4DtAgS1aWO+gU52xpH7M9ScGgXSYmAVS9bIJ8EzuaGw0oNAw==",
      "dev": true,
      "license": "MIT",
      "dependencies": {
        "cross-spawn": "^7.0.1"
      },
      "bin": {
        "cross-env": "src/bin/cross-env.js",
        "cross-env-shell": "src/bin/cross-env-shell.js"
      },
      "engines": {
        "node": ">=10.14",
        "npm": ">=6",
        "yarn": ">=1"
      }
    },
    "node_modules/cross-spawn": {
      "version": "7.0.6",
      "resolved": "https://registry.npmjs.org/cross-spawn/-/cross-spawn-7.0.6.tgz",
      "integrity": "sha512-uV2QOWP2nWzsy2aMp8aRibhi9dlzF5Hgh5SHaB9OiTGEyDTiJJyx0uy51QXdyWbtAHNua4XJzUKca3OzKUd3vA==",
      "dev": true,
      "license": "MIT",
      "dependencies": {
        "path-key": "^3.1.0",
        "shebang-command": "^2.0.0",
        "which": "^2.0.1"
      },
      "engines": {
        "node": ">= 8"
      }
    },
    "node_modules/isexe": {
      "version": "2.0.0",
      "resolved": "https://registry.npmjs.org/isexe/-/isexe-2.0.0.tgz",
      "integrity": "sha512-RHxMLp9lnKHGHRng9QFhRCMbYAcVpn69smSGcq3f36xjgVVWThj4qqLbTLlq7Ssj8B+fIQ1EuCEGI2lKsyQeIw==",
      "dev": true,
      "license": "ISC"
    },
    "node_modules/path-key": {
      "version": "3.1.1",
      "resolved": "https://registry.npmjs.org/path-key/-/path-key-3.1.1.tgz",
      "integrity": "sha512-ojmeN0qd+y0jszEtoY48r0Peq5dwMEkIlCOu6Q5f41lfkswXuKtYrhgoTpLnyIcHm24Uhqx+5Tqm2InSwLhE6Q==",
      "dev": true,
      "license": "MIT",
      "engines": {
        "node": ">=8"
      }
    },
    "node_modules/shebang-command": {
      "version": "2.0.0",
      "resolved": "https://registry.npmjs.org/shebang-command/-/shebang-command-2.0.0.tgz",
      "integrity": "sha512-kHxr2zZpYtdmrN1qDjrrX/Z1rR1kG8Dx+gkpK1G4eXmvXswmcE1hTWBWYUzlraYw1/yZp6YuDY77YtvbN0dmDA==",
      "dev": true,
      "license": "MIT",
      "dependencies": {
        "shebang-regex": "^3.0.0"
      },
      "engines": {
        "node": ">=8"
      }
    },
    "node_modules/shebang-regex": {
      "version": "3.0.0",
      "resolved": "https://registry.npmjs.org/shebang-regex/-/shebang-regex-3.0.0.tgz",
      "integrity": "sha512-7++dFhtcx3353uBaq8DDR4NuxBetBzC7ZQOhmTQInHEd6bSrXdiEyzCvG07Z44UYdLShWUyXt5M/yhz8ekcb1A==",
      "dev": true,
      "license": "MIT",
      "engines": {
        "node": ">=8"
      }
    },
    "node_modules/which": {
      "version": "2.0.2",
      "resolved": "https://registry.npmjs.org/which/-/which-2.0.2.tgz",
      "integrity": "sha512-BLI3Tl1TW3Pvl70l3yq3Y64i+awpwXqsGBYWkkqMtnbXgrMD+yj7rhW0kuEDxzJaYXGjEW5ogapKNMEKNMjibA==",
      "dev": true,
      "license": "ISC",
      "dependencies": {
        "isexe": "^2.0.0"
      },
      "bin": {
        "node-which": "bin/node-which"
      },
      "engines": {
        "node": ">= 8"
      }
    }
  }
}
`

// A native Node static build carries the bex install step between the env
// loader and the exact authored command; a compute build, and a static build
// on another runtime, are unchanged (w8/m86).
func TestNativeStaticBuildInstallsDependenciesFirst(t *testing.T) {
	o := nativeOptions()
	o.StaticSite, o.Runtime, o.StartCommand = true, "", ""
	o.BuildCommand = "npm run build"
	df := nativeDockerfile(o)
	copyAt := strings.Index(df, "COPY --chmod=0755 <<'"+nativeStaticInstallEOF+"' "+nativeStaticInstallPath+"\n")
	run := shellJSON(nativeEnvLoaderPrefix + nativeStaticInstallStep + o.BuildCommand)
	if copyAt < 0 || !strings.Contains(df, run) || strings.Index(df, run) < copyAt || !strings.Contains(df, nativeStaticInstallScript) {
		t.Fatalf("static Dockerfile lacks the install step before the authored command:\n%s", df)
	}
	o.StaticPublishPath = "_site"
	if df := nativeDockerfile(o); !strings.Contains(df, run) {
		t.Fatalf("a site publishing _site lacks the install step:\n%s", df)
	}
	rootPublish := func(p string) Options { r := o; r.StaticPublishPath = p; return r }
	for _, other := range []Options{nativeOptions(), func() Options { p := o; p.Runtime = "python"; return p }(), rootPublish("."), rootPublish("./"), rootPublish("/")} {
		if df := nativeDockerfile(other); strings.Contains(df, nativeStaticInstallPath) {
			t.Errorf("runtime %q static=%v publish %q gained the install step:\n%s", other.Runtime, other.StaticSite, other.StaticPublishPath, df)
		}
	}
}

// TestNativeStaticBuildRunsTheInstallStep is w8/m86's execution gate: the
// generated static image installs an npm project's dependencies (devDependencies
// included, whatever NODE_ENV) before the unchanged build script, with or
// without a lockfile; SKIP_INSTALL_DEPS=true leaves installing to the build
// command; another manager's project and a project with no package.json keep
// their former behavior. Opt-in: needs Docker and network for the npm registry.
func TestNativeStaticBuildRunsTheInstallStep(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build the generated image")
	}
	withLock := map[string]string{"package.json": staticDepsPackage, "package-lock.json": staticDepsLock, "build.js": staticDepsBuild}
	for i, tc := range []struct {
		name, command, publish string
		env, src               map[string]string
		want                   string // the published index, or "" when the build must fail
		log                    string // a line the build log must carry
	}{
		{name: "locked npm project", command: "npm run build", src: withLock, want: "<p>built</p>"},
		{name: "NODE_ENV production keeps build tools", command: "npm run build", env: map[string]string{"NODE_ENV": "production"}, src: withLock, want: "<p>built</p>"},
		{name: "manifest without a lockfile", command: "npm run build", src: map[string]string{"package.json": staticDepsPackage, "build.js": staticDepsBuild}, want: "<p>built</p>"},
		{name: "explicit false installs", command: "npm run build", env: map[string]string{"SKIP_INSTALL_DEPS": "false"}, src: withLock, want: "<p>built</p>"},
		{name: "skip leaves the build command to install", command: "npm run build", env: map[string]string{"SKIP_INSTALL_DEPS": "true"}, src: withLock},
		{name: "skip with the build command installing", command: "npm ci --no-audit --no-fund && npm run build", env: map[string]string{"SKIP_INSTALL_DEPS": "TRUE"}, src: withLock, want: "<p>built</p>"},
		{name: "another manager's lockfile is left alone", command: "npm run build", src: map[string]string{"package.json": staticDepsPackage, "package-lock.json": staticDepsLock, "build.js": staticDepsBuild, "yarn.lock": "# yarn lockfile v1\n"}},
		{name: "no package.json publishes raw files", command: "mkdir -p _site && printf raw > _site/index.html", src: map[string]string{"index.html": "x"}, want: "raw"},
		{name: "a failed install still runs the build command", command: "mkdir -p _site && printf ok > _site/index.html",
			src:  map[string]string{"package.json": `{"name":"x","version":"1.0.0","dependencies":{"bex-no-such-package-m86":"1.0.0"}}`},
			want: "ok", log: "Automatic dependency installation failed"},
		{name: "a root publish path skips the install", command: "npm run build", publish: ".", src: withLock},
		{name: "a narrow PATH still sees packageManager", command: "/bin/mkdir -p _site && echo skipped > _site/index.html",
			env:  map[string]string{"PATH": "/nonexistent"},
			src:  map[string]string{"package.json": `{"name":"x","version":"1.0.0","packageManager":"pnpm@10.34.7"}`, "package-lock.json": staticDepsLock},
			want: "skipped", log: "packageManager pnpm is another package manager's"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := nativeOptions()
			o.StaticSite, o.Runtime, o.StartCommand, o.BuildEnv = true, "", "", nil
			o.BuildCommand, o.StaticPublishPath = tc.command, tc.publish
			// Each case's environment is its own revision, as the operator
			// stamps one per env change: the env is a secret, not a cache key.
			o.NativeEnvRevision = fmt.Sprintf("it-%d-%d", i, os.Getpid())
			tag := fmt.Sprintf("bex-native-static-deps-it-%d-%d", i, os.Getpid())
			out, err := dockerBuildResolved(t, o, tc.env, tc.src, tag)
			if tc.want == "" {
				if err == nil || !strings.Contains(string(out), "cross-env: not found") {
					t.Fatalf("build = %v, want the build script's own 127 on missing cross-env:\n%s", err, out)
				}
				// The tenant's failure summary names their command alone.
				summary := nativeCommandFailure(string(out))
				if strings.Contains(summary, "install-deps || exit") || !strings.Contains(summary, "RUN build command '"+tc.command+"'") ||
					!strings.Contains(summary, "build command '"+tc.command+"' exited with code 127") {
					t.Fatalf("failure summary names bex's step or not the command:\n%s", summary)
				}
				return
			}
			if err != nil {
				t.Fatalf("build: %v\n%s", err, out)
			}
			if !strings.Contains(string(out), tc.log) {
				t.Fatalf("build log lacks %q:\n%s", tc.log, out)
			}
			index, err := exec.Command("docker", "run", "--rm", "--entrypoint", "/bin/cat", tag, "/opt/render/project/src/_site/index.html").CombinedOutput()
			if err != nil || strings.TrimSpace(string(index)) != tc.want {
				t.Fatalf("published index = %v %q, want %q", err, index, tc.want)
			}
		})
	}
}
