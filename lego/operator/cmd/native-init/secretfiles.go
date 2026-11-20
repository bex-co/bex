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

package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// secretFilesDir is where the operator mounts a service's secret files (the
// controller's secretFilesMountPath, the build's nativeSecretFilesDir).
const secretFilesDir = "/etc/secrets"

// linkSecretFiles puts each secret file mounted at secrets in the service
// root as well, where Render also has it: "For non-Docker services, the file
// is also available in your service's root directory" (w8/m154). root is the
// working directory the command starts in, the image's WORKDIR.
//
// The image's build already linked the names it was built with
// (nativeSecretFileLinks). A deploy that reruns that build with other files,
// such as linking an environment group or adding a file to a linked one, mounts
// names the image has no link for, and drops some it has. So every start
// links what is mounted now and removes a link to a file no longer mounted
// (w8/348).
//
// The rules are the build step's: a link to /etc/secrets/<name>, so the root
// copy is read-only and the image's layers never hold the bytes; a repository
// file of the same name gives way to it; a repository directory of that name
// is left alone. Kubelet's own entries in the volume (..data and the
// timestamped directory) are not files. A link that cannot be made is reported
// and skipped: the service still starts, with the file at /etc/secrets.
func linkSecretFiles(secrets, root string, stderr io.Writer) {
	entries, err := os.ReadDir(secrets)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		_, _ = fmt.Fprintf(stderr, "native-init: secret files: %v\n", err)
		return
	}
	mounted := make(map[string]bool, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "..") {
			continue
		}
		mounted[name] = true
		if err := linkSecretFile(secrets, root, name); err != nil {
			_, _ = fmt.Fprintf(stderr, "native-init: secret file %s: %v\n", name, err)
		}
	}
	links, err := os.ReadDir(root)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "native-init: secret files: %v\n", err)
		return
	}
	for _, link := range links {
		name := link.Name()
		if mounted[name] || link.Type()&fs.ModeSymlink == 0 || !linksSecretFile(secrets, root, name) {
			continue
		}
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			_, _ = fmt.Fprintf(stderr, "native-init: secret file %s: %v\n", name, err)
		}
	}
}

// linkSecretFile makes root/name a link to secrets/name, replacing a file or a
// link of that name but not a directory.
func linkSecretFile(secrets, root, name string) error {
	path := filepath.Join(root, name)
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return err
	case info.IsDir():
		return nil
	case linksSecretFile(secrets, root, name):
		return nil
	default:
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return os.Symlink(filepath.Join(secrets, name), path)
}

// linksSecretFile reports whether root/name is the link to the secret file of
// that name, which the build or an earlier start made.
func linksSecretFile(secrets, root, name string) bool {
	target, err := os.Readlink(filepath.Join(root, name))
	return err == nil && target == filepath.Join(secrets, name)
}
