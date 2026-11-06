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

package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func filesCommand(d deps) *cobra.Command {
	var (
		direction string
		turn      int
		into      string
		force     bool
	)
	c := &cobra.Command{
		Use:   "files <ags-id>",
		Short: "List or download the files of an agent session",
		Long: "Lists the files the agent shared (default), the files attached to the\n" +
			"session (--direction input), or both (--direction all). --download DIR\n" +
			"saves them, checking each against its SHA-256.",
		Example: "  bex agent files ags-…\n  bex agent files ags-… --turn 2 --download ./out",
		Args:    sessionArgs(0),
		RunE: func(c *cobra.Command, args []string) error {
			switch direction {
			case "input", "output":
			case "all":
				direction = ""
			default:
				return usageError("--direction must be output, input or all")
			}
			if turn < 0 {
				return usageError("--turn must be a positive turn number")
			}
			inv, cancel, err := d.start(c)
			if err != nil {
				return err
			}
			defer cancel()
			files, raws, err := inv.api.attachments(inv.ctx, args[0], direction, turn)
			if err != nil {
				return err
			}
			if into == "" {
				if inv.format != formatText {
					return writeRaw(inv.out, inv.format, rawArray(raws))
				}
				writeAttachments(inv.out, files)
				return nil
			}
			saved, err := inv.downloadAll(files, into, force)
			if err != nil {
				return err
			}
			if inv.format != formatText {
				encoded, err := json.Marshal(saved)
				if err != nil {
					return err
				}
				return writeRaw(inv.out, inv.format, encoded)
			}
			for _, s := range saved {
				_, _ = fmt.Fprintf(inv.out, "%s  %s\n", s.ID, s.Path)
			}
			if len(saved) == 0 {
				_, _ = fmt.Fprintln(inv.out, "No files.")
			}
			return nil
		},
	}
	c.Flags().StringVar(&direction, "direction", "output", "output (shared by the agent), input (attached) or all")
	c.Flags().IntVar(&turn, "turn", 0, "only this turn's files")
	c.Flags().StringVar(&into, "download", "", "save the files into this directory")
	c.Flags().BoolVar(&force, "force", false, "overwrite files that already exist in the --download directory")
	return c
}

// savedFile is one downloaded file, for -o json.
type savedFile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

func (inv *invocation) downloadAll(files []attachment, dir string, force bool) ([]savedFile, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	taken := map[string]bool{}
	saved := make([]savedFile, 0, len(files))
	for _, f := range files {
		name := localName(f, taken)
		path := filepath.Join(dir, name)
		if err := inv.download(f, path, force); err != nil {
			return saved, fmt.Errorf("download %s (%s): %w", f.Name, f.ID, err)
		}
		saved = append(saved, savedFile{ID: f.ID, Name: f.Name, Path: path})
	}
	return saved, nil
}

// localName is the file's own base name, suffixed with its id when another
// file in this download already took it (two turns can share a name).
func localName(f attachment, taken map[string]bool) string {
	name := filepath.Base(filepath.Clean("/" + f.Name))
	if name == "/" || name == "." || name == "" {
		name = f.ID
	}
	if taken[name] {
		ext := filepath.Ext(name)
		name = strings.TrimSuffix(name, ext) + "-" + f.ID + ext
	}
	taken[name] = true
	return name
}

// download writes one attachment to path through a temporary file, so an
// interrupted or corrupt transfer never leaves a partial file under the
// final name.
func (inv *invocation) download(f attachment, path string, force bool) error {
	if !force {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", path)
		}
	}
	signed := f.DownloadURL
	if signed == "" {
		raw, err := inv.api.getJSON(inv.ctx, nil, "agent-attachments", f.ID)
		if err != nil {
			return err
		}
		var fresh attachment
		if err := json.Unmarshal(raw, &fresh); err != nil {
			return err
		}
		signed = fresh.DownloadURL
	}
	if signed == "" {
		return fmt.Errorf("the API returned no download URL")
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".bex-download-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	hash := sha256.New()
	if err := inv.api.fetch(inv.ctx, signed, io.MultiWriter(tmp, hash)); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if f.SHA256 != "" && !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), f.SHA256) {
		return fmt.Errorf("the downloaded bytes do not match the file's SHA-256")
	}
	return os.Rename(tmp.Name(), path)
}
