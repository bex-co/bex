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

package s3

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/render-oss/cli/pkg/command"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// format is the resolved --output: JSON and YAML print the API's own answer,
// text (and upstream's interactive default) a human summary.
type format string

const (
	formatText format = "text"
	formatJSON format = "json"
	formatYAML format = "yaml"
)

// resolveFormat is the command's --output. The root pre-run resolves it with
// RENDER_OUTPUT and the terminal check, as for every other command; the raw
// flag is the fallback when no pre-run ran.
func resolveFormat(c *cobra.Command) (format, error) {
	value, _ := c.Flags().GetString("output")
	if resolved := command.GetFormatFromContext(c.Context()); resolved != nil {
		value = string(*resolved)
	}
	switch strings.ToLower(value) {
	case "", "interactive", "text":
		return formatText, nil
	case "json":
		return formatJSON, nil
	case "yaml":
		return formatYAML, nil
	}
	return "", usageError("--output must be json, yaml or text, not %q", value)
}

// writeValue prints v as JSON or YAML; the caller prints text itself.
func writeValue(w io.Writer, f format, v any) error {
	encoded, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return writeRaw(w, f, encoded)
}

// writeRaw prints an API answer as JSON (indented) or YAML.
func writeRaw(w io.Writer, f format, raw []byte) error {
	if f == formatYAML {
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return err
		}
		out, err := yaml.Marshal(decoded)
		if err != nil {
			return err
		}
		_, err = w.Write(out)
		return err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return err
	}
	buf.WriteByte('\n')
	_, err := w.Write(buf.Bytes())
	return err
}

// table is a two-column label/value block that skips empty values.
type table struct{ *tabwriter.Writer }

func newTable(w io.Writer) table { return table{tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)} }

func (t table) row(label, value string) {
	if value != "" {
		_, _ = fmt.Fprintf(t, "%s\t%s\n", label, value)
	}
}

// minuteLayout is how text output shows a time.
const minuteLayout = "2006-01-02 15:04 MST"

// timeText is an RFC 3339 time from the API in UTC minutes, "" when absent.
func timeText(raw *string) string {
	if raw == nil || *raw == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, *raw)
	if err != nil {
		return *raw
	}
	return t.UTC().Format(minuteLayout)
}

// humanBytes is a size in binary units.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
