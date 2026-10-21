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

// Command seccomp-render-v1 prints the bex-render-v1 seccomp profile for the
// architecture it runs on (w6/m150): containerd's own RuntimeDefault for the
// render-v1 capability set, plus the user-namespace and io_uring syscalls
// Render's unfiltered runtime permits. generate.sh runs it per architecture
// and merges the results.
package main

import (
	"encoding/json"
	"os"

	"github.com/containerd/containerd/v2/contrib/seccomp"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// renderV1Capabilities mirrors appv1alpha1.ContainerPolicyCapabilities(render-v1).
var renderV1Capabilities = []string{"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FOWNER", "CAP_SETGID", "CAP_SETUID", "CAP_SYS_CHROOT"}

// userNamespaceSyscalls are what bubblewrap and the Chromium sandbox call
// inside a user namespace they created. RuntimeDefault grants them only with
// CAP_SYS_ADMIN; the kernel already scopes each to namespaces the caller owns.
var userNamespaceSyscalls = []string{
	"unshare", "setns", "mount", "umount", "umount2", "pivot_root",
	"mount_setattr", "move_mount", "open_tree", "fsopen", "fsconfig", "fsmount", "fspick",
	"sethostname", "setdomainname",
}

var ioUringSyscalls = []string{"io_uring_setup", "io_uring_enter", "io_uring_register"}

func main() {
	caps := renderV1Capabilities
	profile := seccomp.DefaultProfile(&specs.Spec{Process: &specs.Process{
		Capabilities: &specs.LinuxCapabilities{Bounding: caps, Effective: caps, Permitted: caps},
	}})
	// clone without RuntimeDefault's namespace-flag mask: creating a user
	// namespace (and namespaces nested in it) is what Render permits.
	for i := range profile.Syscalls {
		if len(profile.Syscalls[i].Names) == 1 && profile.Syscalls[i].Names[0] == "clone" {
			profile.Syscalls[i].Args = nil
		}
	}
	profile.Syscalls = append(profile.Syscalls,
		specs.LinuxSyscall{Names: userNamespaceSyscalls, Action: specs.ActAllow},
		specs.LinuxSyscall{Names: ioUringSyscalls, Action: specs.ActAllow},
	)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(profile); err != nil {
		os.Exit(1)
	}
}
