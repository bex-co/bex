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
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Commands returns every Bex-native coding command: one launcher per catalog
// provider (`bex glm`, `bex muse`, …) plus the `bex code` management hub.
func Commands() []*cobra.Command {
	commands := []*cobra.Command{newCodeCmd()}
	for _, p := range Catalog() {
		commands = append(commands, newLaunchCmd(p))
	}
	return commands
}

// newLaunchCmd builds a provider launcher. Flag parsing is disabled: Claude
// Code owns every argument after the provider name.
func newLaunchCmd(p Provider) *cobra.Command {
	return &cobra.Command{
		Use:   p.Name + " [claude arguments]",
		Short: "Launch Claude Code backed by " + p.DisplayName,
		Long: "Starts a Claude Code instance connected to " + p.DisplayName + "\n" +
			"(" + p.BaseURL + ", default model " + p.DefaultModel + ") in an isolated\n" +
			"configuration directory, so every provider keeps its own settings and\n" +
			"history. The API key is captured once on first launch (hidden paste,\n" +
			"verified live) and stored owner-only; the environment (" + strings.Join(p.KeyEnvs, ", ") + ")\n" +
			"always overrides it. All arguments pass through to claude unchanged.",
		DisableFlagParsing: true,
		SilenceUsage:       true,
		RunE: func(c *cobra.Command, args []string) error {
			return launch(p, args, c.ErrOrStderr())
		},
	}
}

func newCodeCmd() *cobra.Command {
	code := &cobra.Command{
		Use:   "code",
		Short: "Manage the Bex coding-agent launchers (" + providerNames() + ")",
		Long: "`bex " + strings.ReplaceAll(providerNames(), ", ", "`, `bex ") + "` each launch a Claude Code\n" +
			"instance connected to that provider's Anthropic-compatible endpoint, in\n" +
			"an isolated per-provider configuration directory under ~/.bex/code\n" +
			"(override with BEX_CODE_HOME). A provider's API key is captured on its\n" +
			"first launch — pasted hidden, verified live, stored owner-only in\n" +
			"~/.bex/code/keys.toml — and the environment always overrides the store.\n" +
			"\n" +
			"`bex code` shows provider status; `bex code keys` manages the keys.",
		RunE: func(c *cobra.Command, args []string) error {
			return printStatus(c.OutOrStdout())
		},
	}
	keys := &cobra.Command{
		Use:   "keys",
		Short: "Show which provider API keys are configured",
		RunE: func(c *cobra.Command, args []string) error {
			return printStatus(c.OutOrStdout())
		},
	}
	keys.AddCommand(&cobra.Command{
		Use:   "set <provider>",
		Short: "Capture and verify a provider API key (hidden paste; or piped stdin)",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			p := findProvider(args[0])
			if p == nil {
				return fmt.Errorf("unknown provider %q; one of %s", args[0], providerNames())
			}
			base, err := baseDir(os.LookupEnv, os.UserHomeDir)
			if err != nil {
				return err
			}
			key, err := readSecret(c.ErrOrStderr(), fmt.Sprintf("Paste the %s API key (input hidden): ", p.Name))
			if err != nil {
				return err
			}
			return verifyAndStoreKey(*p, keysPath(base), key, c.ErrOrStderr())
		},
	})
	keys.AddCommand(&cobra.Command{
		Use:   "unset <provider>",
		Short: "Remove a stored provider API key",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			base, err := baseDir(os.LookupEnv, os.UserHomeDir)
			if err != nil {
				return err
			}
			stored, err := loadStoredKeys(keysPath(base))
			if err != nil {
				return err
			}
			if _, ok := stored[args[0]]; !ok {
				return fmt.Errorf("no stored key for %q", args[0])
			}
			delete(stored, args[0])
			return saveStoredKeys(keysPath(base), stored)
		},
	})
	code.AddCommand(keys)
	return code
}

func printStatus(w io.Writer) error {
	base, err := baseDir(os.LookupEnv, os.UserHomeDir)
	if err != nil {
		return err
	}
	stored, err := loadStoredKeys(keysPath(base))
	if err != nil {
		return err
	}
	for _, p := range Catalog() {
		status := ""
		switch _, source := providerKey(p, os.LookupEnv, stored); source {
		case "":
			status = fmt.Sprintf("no key — `bex %s` captures one, or `bex code keys set %s`", p.Name, p.Name)
		case "stored":
			status = "key stored"
		default:
			status = fmt.Sprintf("key from %s (environment)", source)
		}
		_, _ = fmt.Fprintf(w, "bex %-8s  %-28s  %s\n", p.Name, p.DisplayName, status)
		_, _ = fmt.Fprintf(w, "              default %s · %s\n", p.DefaultModel, p.BaseURL)
	}
	return nil
}
