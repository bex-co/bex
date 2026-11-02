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

package desktop

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"

	"github.com/render-oss/cli/pkg/utils"
)

// lineSuffix is the editor's `path:line[:col]` position, stripped before the
// existence check.
var lineSuffix = regexp.MustCompile(`(:\d+){1,2}$`)

// Shorthand rewrites `bex <path> …` to `bex edit <path> …` when the first
// argument looks like a path and exists. Anything else — a bare word, a flag,
// a URL, an empty argument list — is returned unchanged for cobra, so the
// shorthand can never shadow a command: `bex docs` stays the docs command
// even beside a docs/ folder. A path-shaped argument that does not exist is
// an error rather than a new file; `bex edit` creates on purpose.
func Shorthand(args []string) ([]string, error) {
	return shorthand(args, os.Stat)
}

func shorthand(args []string, stat func(string) (os.FileInfo, error)) ([]string, error) {
	if len(args) == 0 || !pathShaped(args[0]) {
		return args, nil
	}
	// A real file may be named `notes:2024`; only strip a position when the
	// argument as written does not exist.
	for _, candidate := range []string{args[0], lineSuffix.ReplaceAllString(args[0], "")} {
		path, err := utils.ExpandHome(candidate)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", args[0], err)
		}
		if _, err := stat(path); err == nil {
			// Forward the expanded path: a quoted ~ reaches the launcher
			// literally, and it does not expand it.
			return append([]string{"edit", path + strings.TrimPrefix(args[0], candidate)}, args[1:]...), nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	missing := lineSuffix.ReplaceAllString(args[0], "")
	return nil, fmt.Errorf("no such file or directory: %s (to create it, run `bex edit %s`)", args[0], missing)
}

// pathShaped is the whole shorthand rule: `.`, `..`, `~`, or any argument
// containing a slash (which covers ./ ../ / ~/) — never a URL or a flag. No
// command name has that shape, which is why the shorthand cannot shadow one.
func pathShaped(arg string) bool {
	if arg == "" || strings.HasPrefix(arg, "-") || strings.Contains(arg, "://") {
		return false
	}
	switch arg {
	case ".", "..", "~":
		return true
	}
	return strings.Contains(arg, "/")
}
