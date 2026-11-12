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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// --direction values: the API's attachment directions, plus all for both.
const (
	directionOutput = "output"
	directionInput  = "input"
	directionAll    = "all"
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
			case directionInput, directionOutput:
			case directionAll:
				direction = ""
			default:
				return usageError("--direction must be %s, %s or %s", directionOutput, directionInput, directionAll)
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
			// The files that landed are listed even when a later one failed.
			saved, err := inv.downloadAll(files, into, force)
			if printErr := inv.printSaved(saved, err == nil); printErr != nil {
				return printErr
			}
			return err
		},
	}
	c.Flags().StringVar(&direction, "direction", directionOutput, directionOutput+" (shared by the agent), "+directionInput+" (attached) or "+directionAll)
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

func (inv *invocation) printSaved(saved []savedFile, complete bool) error {
	if inv.format != formatText {
		return writeValue(inv.out, inv.format, saved)
	}
	for _, s := range saved {
		_, _ = fmt.Fprintf(inv.out, "%s  %s\n", s.ID, s.Path)
	}
	if len(saved) == 0 && complete {
		_, _ = fmt.Fprintln(inv.out, "No files.")
	}
	return nil
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
// file in this download already took it (two turns can share a name). Names
// are compared case-insensitively: on macOS and Windows A.txt is a.txt.
func localName(f attachment, taken map[string]bool) string {
	name := filepath.Base(filepath.Clean("/" + f.Name))
	if name == "/" || name == "." || name == "" {
		name = f.ID
	}
	if taken[strings.ToLower(name)] {
		ext := filepath.Ext(name)
		name = strings.TrimSuffix(name, ext) + "-" + f.ID + ext
	}
	taken[strings.ToLower(name)] = true
	return name
}

// freshURL reads the attachment again for a new signed download URL.
func (inv *invocation) freshURL(id string) (string, error) {
	raw, err := inv.api.getJSON(inv.ctx, nil, "agent-attachments", id)
	if err != nil {
		return "", err
	}
	var fresh attachment
	if err := json.Unmarshal(raw, &fresh); err != nil {
		return "", err
	}
	if fresh.DownloadURL == "" {
		return "", errors.New("the API returned no download URL")
	}
	return fresh.DownloadURL, nil
}

// download writes one attachment to path through a temporary file, so an
// interrupted or corrupt transfer never leaves a partial file under the
// final name.
func (inv *invocation) download(f attachment, path string, force bool) error {
	if !force {
		// A cheap early refusal; place re-checks atomically.
		if _, err := os.Lstat(path); err == nil {
			return existsError(path)
		}
	}
	signed := f.DownloadURL
	if signed == "" {
		var err error
		if signed, err = inv.freshURL(f.ID); err != nil {
			return err
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".bex-download-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	hash := sha256.New()
	err = inv.api.fetch(inv.ctx, signed, io.MultiWriter(tmp, hash))
	var dlErr *downloadError
	if errors.As(err, &dlErr) && dlErr.status == http.StatusForbidden {
		// The listing's signed URLs last 15 minutes (ADR095), which a long
		// batch can outlive: ask for a new one and start this file over.
		if signed, err = inv.freshURL(f.ID); err == nil {
			if err = rewind(tmp); err == nil {
				hash.Reset()
				err = inv.api.fetch(inv.ctx, signed, io.MultiWriter(tmp, hash))
			}
		}
	}
	if err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if f.SHA256 != "" && !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), f.SHA256) {
		return errors.New("the downloaded bytes do not match the file's SHA-256")
	}
	return place(tmp.Name(), path, force)
}

func existsError(path string) error {
	return fmt.Errorf("%s already exists (use --force to overwrite)", path)
}

func rewind(f *os.File) error {
	if err := f.Truncate(0); err != nil {
		return err
	}
	_, err := f.Seek(0, io.SeekStart)
	return err
}

// place moves the finished download to path. Without force a hard link
// creates path only if it does not exist, so a file that appeared during the
// download is never replaced; a filesystem without hard links falls back to
// a checked rename.
func place(tmp, path string, force bool) error {
	if force {
		return os.Rename(tmp, path)
	}
	err := os.Link(tmp, path)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fs.ErrExist):
		return existsError(path)
	}
	if _, statErr := os.Lstat(path); statErr == nil {
		return existsError(path)
	}
	return os.Rename(tmp, path)
}
