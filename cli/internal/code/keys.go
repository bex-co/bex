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

package code

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// baseDir is where bex code keeps everything: keys.toml plus one isolated
// claude-<provider> configuration directory per provider. BEX_CODE_HOME
// overrides the default ~/.bex/code.
func baseDir(lookup func(string) (string, bool), userHome func() (string, error)) (string, error) {
	if v, ok := lookup("BEX_CODE_HOME"); ok && v != "" {
		return v, nil
	}
	home, err := userHome()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".bex", "code"), nil
}

func keysPath(base string) string {
	return filepath.Join(base, "keys.toml")
}

// loadStoredKeys reads the key store. A missing file is an empty store, not
// an error. The format is what saveStoredKeys writes — `name = "value"`
// lines plus comments — so parsing is deliberately narrow.
func loadStoredKeys(path string) (map[string]string, error) {
	keys := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return keys, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	for lineNumber, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, quoted, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("%s:%d: expected `name = \"key\"`", path, lineNumber+1)
		}
		value, err := strconv.Unquote(strings.TrimSpace(quoted))
		if err != nil {
			return nil, fmt.Errorf("%s:%d: expected a quoted key: %w", path, lineNumber+1, err)
		}
		keys[strings.TrimSpace(name)] = value
	}
	return keys, nil
}

// saveStoredKeys writes the key store owner-only, catalog providers first for
// a stable file, any unknown names after them.
func saveStoredKeys(path string, keys map[string]string) error {
	var b strings.Builder
	b.WriteString("# Stored by `bex code keys set` — provider API keys, owner-only.\n")
	written := map[string]bool{}
	for _, p := range Catalog() {
		if v, ok := keys[p.Name]; ok {
			fmt.Fprintf(&b, "%s = %s\n", p.Name, strconv.Quote(v))
			written[p.Name] = true
		}
	}
	var rest []string
	for name := range keys {
		if !written[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	for _, name := range rest {
		fmt.Fprintf(&b, "%s = %s\n", name, strconv.Quote(keys[name]))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return os.Chmod(path, 0o600)
}

// providerKey returns the effective key for a provider and its source: the
// winning environment-variable name, "stored" for the key file, or "".
func providerKey(p Provider, lookup func(string) (string, bool), stored map[string]string) (key, source string) {
	for _, name := range p.KeyEnvs {
		if v, ok := lookup(name); ok && v != "" {
			return v, name
		}
	}
	if v := stored[p.Name]; v != "" {
		return v, "stored"
	}
	return "", ""
}
