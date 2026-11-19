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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// bucketName is the rule of ADR112 D1: a strict subset of S3's. Checking it
// here means a name typed on the command line can only name a bucket, never
// another route.
var bucketName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)

// accessKeyID is an `s3k-` typed id (ADR020).
var accessKeyID = regexp.MustCompile(`^s3k-[0-9a-z]{20}$`)

// bucketArg is the bucket a command names, as `photos` or `s3://photos`.
func bucketArg(arg string) (string, error) {
	name := strings.TrimSuffix(strings.TrimPrefix(arg, "s3://"), "/")
	if !bucketName.MatchString(name) {
		return "", usageError("%q is not a bucket name: 3 to 63 lowercase letters, digits and hyphens, starting and ending with a letter or digit", arg)
	}
	return name, nil
}

// shellQuote quotes value for POSIX sh.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// ---- env -------------------------------------------------------------------

// envVar is one variable of `bex s3 env`.
type envVar struct{ Name, Value string }

// envVars are the settings for the tools that read the environment. aws-cli
// and the AWS SDKs read the AWS_ names; rclone reads RCLONE_S3_ and defaults
// to path-style addressing; s5cmd reads S3_ENDPOINT_URL.
func envVars(s session) []envVar {
	return []envVar{
		{"AWS_ACCESS_KEY_ID", s.AccessKeyID},
		{"AWS_SECRET_ACCESS_KEY", s.Secret},
		{"AWS_REGION", s.Region},
		{"AWS_ENDPOINT_URL_S3", s.Endpoint},
		{"S3_ENDPOINT_URL", s.Endpoint},
		{"RCLONE_S3_PROVIDER", "Other"},
		{"RCLONE_S3_ENDPOINT", s.Endpoint},
		{"RCLONE_S3_ACCESS_KEY_ID", s.AccessKeyID},
		{"RCLONE_S3_SECRET_ACCESS_KEY", s.Secret},
		{"RCLONE_S3_REGION", s.Region},
	}
}

// envNotes are the settings no environment variable carries.
var envNotes = []string{
	"bex serves path-style addressing only (https://objects.<domain>/<bucket>/<key>).",
	"boto3: boto3.client(\"s3\", config=botocore.config.Config(s3={\"addressing_style\": \"path\"})), or addressing_style = path under s3 in the AWS config.",
	"Terraform's AWS provider: s3_use_path_style = true (not tested against bex; some bucket attributes it reads answer 501).",
	"aws-cli: `bex s3 ...` sets all of this for you.",
}

type envOutput struct {
	Workspace     string            `json:"workspace"`
	Endpoint      string            `json:"endpoint"`
	Region        string            `json:"region"`
	AccessKeyID   string            `json:"accessKeyId"`
	Secret        string            `json:"secretAccessKey"`
	ExpiresAt     string            `json:"expiresAt"`
	Variables     map[string]string `json:"variables"`
	AWSConfigFile string            `json:"awsConfigFile"`
	Notes         []string          `json:"notes"`
}

func envCommand(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "env",
		Short: "Print the endpoint and a key as variables for rclone, boto3, s5cmd and Terraform",
		Long: "Prints the S3 endpoint and the twelve-hour key `bex s3` uses as environment\n" +
			"variables, for the tools that are not the AWS CLI: aws-sdk programs and boto3,\n" +
			"rclone and s5cmd. Load them into a shell with:\n\n" +
			"  eval \"$(bex s3 env)\"\n\n" +
			"The key expires (the output says when); run it again for a new one. A key that\n" +
			"does not expire, for CI, comes from `bex s3 keys create`.\n\n" +
			"Use -o json for the values as data.",
		Example: "  eval \"$(bex s3 env)\"\n  rclone ls :s3:photos",
		Args:    cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			f, err := resolveFormat(c)
			if err != nil {
				return err
			}
			s, err := d.session(commandContext(c))
			if err != nil {
				return err
			}
			vars := envVars(s)
			if f != formatText {
				values := make(map[string]string, len(vars))
				for _, v := range vars {
					values[v.Name] = v.Value
				}
				return writeValue(c.OutOrStdout(), f, envOutput{
					Workspace: s.Workspace, Endpoint: s.Endpoint, Region: s.Region, AccessKeyID: s.AccessKeyID, Secret: s.Secret,
					ExpiresAt: s.ExpiresAt.UTC().Format(time.RFC3339), Variables: values, AWSConfigFile: s.ConfigPath, Notes: envNotes,
				})
			}
			out := c.OutOrStdout()
			_, _ = fmt.Fprintf(out, "# bex S3 for workspace %s; this key expires %s (`bex s3 keys create` makes one that does not).\n",
				s.Workspace, s.ExpiresAt.UTC().Format(time.RFC3339))
			for _, v := range vars {
				_, _ = fmt.Fprintf(out, "export %s=%s\n", v.Name, shellQuote(v.Value))
			}
			for _, note := range envNotes {
				_, _ = fmt.Fprintf(out, "# %s\n", note)
			}
			return nil
		},
	}
}

// ---- keys ------------------------------------------------------------------

func keysCommand(d deps) *cobra.Command {
	keys := &cobra.Command{
		Use:   "keys",
		Short: "Manage the workspace's S3 access keys",
		Long: "Persistent S3 access keys are for CI, rclone and Terraform: they do not expire\n" +
			"and are shown once. `bex s3` itself uses a twelve-hour key it makes for you, so\n" +
			"you need these only for a use that outlives that. Creating or revoking a key\n" +
			"needs a person's login (`bex login`); an API key cannot do either. A key acts\n" +
			"as the person who made it, in this workspace, and is revoked with them.",
		SilenceUsage: true,
	}
	keys.AddCommand(keysCreateCommand(d), keysListCommand(d), keysRevokeCommand(d))
	return keys
}

func keysCreateCommand(d deps) *cobra.Command {
	var name, permission string
	var expiresIn time.Duration
	c := &cobra.Command{
		Use:   "create",
		Short: "Create an S3 access key; its secret is printed once",
		Long: "Creates an S3 access key for this workspace and prints its id and secret. The\n" +
			"secret is shown once and bex cannot show it again. A key does not expire unless\n" +
			"you pass --expires-in (at most 12h). --permission read can only list and read.",
		Example: "  bex s3 keys create --name ci\n  bex s3 keys create --name backup-reader --permission read",
		Args:    cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if permission != "read" && permission != "readwrite" {
				return usageError("--permission must be read or readwrite, not %q", permission)
			}
			if expiresIn < 0 || expiresIn > keyLife {
				return usageError("--expires-in must be between 0 (the key does not expire) and %s", keyLife)
			}
			f, err := resolveFormat(c)
			if err != nil {
				return err
			}
			owner, err := d.owner()
			if err != nil {
				return err
			}
			a, err := d.api()
			if err != nil {
				return err
			}
			key, raw, err := a.createKey(commandContext(c), owner, createKeyRequest{
				Name: name, Permission: permission, ExpiresInSeconds: int64(expiresIn / time.Second),
			})
			if err != nil {
				return mintError(owner, err)
			}
			if f != formatText {
				return writeRaw(c.OutOrStdout(), f, raw)
			}
			kind := "persistent"
			if key.ExpiresAt != nil {
				kind = "ephemeral, expires " + timeText(key.ExpiresAt)
			}
			out := c.OutOrStdout()
			_, _ = fmt.Fprintf(out, "Created %s S3 access key %s (%s) for workspace %s.\n\n", kind, key.ID, key.Permission, owner)
			t := newTable(out)
			t.row("Access key ID", key.ID)
			t.row("Secret access key", key.Secret)
			_ = t.Flush()
			_, _ = fmt.Fprintln(out, "\nThe secret is shown once; bex cannot show it again. `bex s3 env` prints the settings for other tools.")
			return nil
		},
	}
	c.Flags().StringVar(&name, "name", "bex s3 key", "a name that tells the keys apart in `keys list`")
	c.Flags().StringVar(&permission, "permission", "readwrite", "read (list and get) or readwrite")
	c.Flags().DurationVar(&expiresIn, "expires-in", 0, "make an expiring key, at most 12h (default: the key does not expire)")
	return c
}

func keysListCommand(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the workspace's S3 access keys",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			f, err := resolveFormat(c)
			if err != nil {
				return err
			}
			owner, err := d.owner()
			if err != nil {
				return err
			}
			a, err := d.api()
			if err != nil {
				return err
			}
			raw, err := a.do(commandContext(c), http.MethodGet, nil, keysPath(owner)...)
			if err != nil {
				return fmt.Errorf("list S3 access keys in %s: %w", owner, err)
			}
			if f != formatText {
				return writeRaw(c.OutOrStdout(), f, raw)
			}
			var keys []accessKey
			if err := json.Unmarshal(raw, &keys); err != nil {
				return fmt.Errorf("decode the access keys: %w", err)
			}
			out := c.OutOrStdout()
			if len(keys) == 0 {
				_, _ = fmt.Fprintln(out, "No S3 access keys. `bex s3` makes its own when you use it; `bex s3 keys create` makes a persistent one.")
				return nil
			}
			t := newTable(out)
			_, _ = fmt.Fprintln(t, "ID\tNAME\tPERMISSION\tSTATUS\tEXPIRES\tLAST USED")
			for _, k := range keys {
				expires := timeText(k.ExpiresAt)
				if expires == "" {
					expires = "never"
				}
				used := timeText(k.LastUsedAt)
				if used == "" {
					used = "never"
				}
				_, _ = fmt.Fprintf(t, "%s\t%s\t%s\t%s\t%s\t%s\n", k.ID, k.Name, k.Permission, k.Status, expires, used)
			}
			return t.Flush()
		},
	}
}

func keysRevokeCommand(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <key id>…",
		Short: "Revoke S3 access keys; they stop working within 30 seconds",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageError("expected one or more access key ids (s3k-…); `bex s3 keys list` shows them")
			}
			for _, id := range args {
				if !accessKeyID.MatchString(id) {
					return usageError("%q is not an access key id (s3k-…)", id)
				}
			}
			return nil
		},
		RunE: func(c *cobra.Command, args []string) error {
			f, err := resolveFormat(c)
			if err != nil {
				return err
			}
			owner, err := d.owner()
			if err != nil {
				return err
			}
			a, err := d.api()
			if err != nil {
				return err
			}
			revoked := []string{}
			for _, id := range args {
				if _, err := a.do(commandContext(c), http.MethodDelete, nil, keysPath(owner, id)...); err != nil {
					return fmt.Errorf("revoke %s: %w", id, err)
				}
				revoked = append(revoked, id)
				if dir, err := d.stateDir(); err == nil {
					forgetKey(dir, id)
				}
				if f == formatText {
					_, _ = fmt.Fprintf(c.OutOrStdout(), "Revoked %s\n", id)
				}
			}
			if f != formatText {
				return writeValue(c.OutOrStdout(), f, map[string][]string{"revoked": revoked})
			}
			return nil
		},
	}
}

// forgetKey drops a cached key that was just revoked, so the next `bex s3`
// makes a new one instead of failing with a 403.
func forgetKey(dir, id string) {
	paths, _ := filepath.Glob(filepath.Join(dir, "key-*.json"))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var key cachedKey
		if json.Unmarshal(raw, &key) == nil && key.AccessKeyID == id {
			_ = os.Remove(path)
		}
	}
}

// ---- public and info -------------------------------------------------------

func publicCommand(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "public <bucket> enable|disable|status",
		Short: "Publish a bucket for anyone to read, stop publishing it, or show whether it is public",
		Long: "`enable` serves the bucket read-only at https://pub-<id>.<hosting domain>/<key>,\n" +
			"with no credential: anyone on the internet can read every object in it. `disable`\n" +
			"stops serving it (within about 30 seconds); `status` shows whether it is public.\n" +
			"Publishing needs the workspace's admin role and your own login: an S3 access key\n" +
			"cannot publish a bucket, and there is no S3 bucket policy for it.\n\n" +
			"Public reads count against the egress bex shares across all workspaces and are\n" +
			"rate limited; the host is not a CDN.",
		Example: "  bex s3 public photos enable\n  bex s3 public photos status",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 2 {
				return usageError("expected a bucket and enable, disable or status")
			}
			switch args[1] {
			case "enable", "disable", "status":
				return nil
			}
			return usageError("%q is not enable, disable or status", args[1])
		},
		RunE: func(c *cobra.Command, args []string) error {
			bucket, err := bucketArg(args[0])
			if err != nil {
				return err
			}
			f, err := resolveFormat(c)
			if err != nil {
				return err
			}
			owner, err := d.owner()
			if err != nil {
				return err
			}
			a, err := d.api()
			if err != nil {
				return err
			}
			method, body := http.MethodGet, any(nil)
			if args[1] != "status" {
				method, body = http.MethodPut, map[string]bool{"enabled": args[1] == "enable"}
			}
			raw, err := a.do(commandContext(c), method, body, bucketPath(owner, bucket, "public")...)
			if err != nil {
				return bucketError(owner, bucket, args[1] != "status", err)
			}
			if f != formatText {
				return writeRaw(c.OutOrStdout(), f, raw)
			}
			var got bucketPublic
			if err := json.Unmarshal(raw, &got); err != nil {
				return fmt.Errorf("decode the answer: %w", err)
			}
			out := c.OutOrStdout()
			if !got.Public {
				_, _ = fmt.Fprintf(out, "%s is private.\n", bucket)
				return nil
			}
			_, _ = fmt.Fprintf(out, "%s is public: anyone can read its objects at https://%s/<key>\n", bucket, got.Host)
			if args[1] == "enable" {
				_, _ = fmt.Fprintf(out, "Public reads count against bex's shared egress and are rate limited. `bex s3 public %s disable` stops serving within about 30 seconds.\n", bucket)
			}
			return nil
		},
	}
}

func infoCommand(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "info <bucket>",
		Short: "Show a bucket's region, public host, objects and bytes",
		Long: "Shows what S3 cannot answer: the bucket's region, whether and where it is public,\n" +
			"and its objects and bytes as of the last hourly count (a count can be older than\n" +
			"the last upload).",
		Example: "  bex s3 info photos",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageError("expected one bucket")
			}
			return nil
		},
		RunE: func(c *cobra.Command, args []string) error {
			bucket, err := bucketArg(args[0])
			if err != nil {
				return err
			}
			f, err := resolveFormat(c)
			if err != nil {
				return err
			}
			owner, err := d.owner()
			if err != nil {
				return err
			}
			a, err := d.api()
			if err != nil {
				return err
			}
			raw, err := a.do(commandContext(c), http.MethodGet, nil, bucketPath(owner, bucket)...)
			if err != nil {
				return bucketError(owner, bucket, false, err)
			}
			if f != formatText {
				return writeRaw(c.OutOrStdout(), f, raw)
			}
			var b bucketInfo
			if err := json.Unmarshal(raw, &b); err != nil {
				return fmt.Errorf("decode the bucket: %w", err)
			}
			t := newTable(c.OutOrStdout())
			t.row("Bucket", b.Name)
			t.row("Region", b.Region)
			if b.Public {
				t.row("Public", "yes, https://"+b.PublicHost+"/<key>")
			} else {
				t.row("Public", "no")
			}
			if b.ObservedAt == nil {
				t.row("Objects", "not counted yet")
			} else {
				t.row("Objects", fmt.Sprintf("%d", b.Objects))
				t.row("Size", fmt.Sprintf("%s (counted %s)", humanBytes(b.Bytes), timeText(b.ObservedAt)))
			}
			if b.System {
				t.row("Kind", "system bucket (Render's `ea objects` namespace; it cannot be deleted)")
			}
			created := b.CreatedAt
			t.row("Created", timeText(&created))
			return t.Flush()
		},
	}
}

// bucketError says what a refused bucket route needs.
func bucketError(owner, bucket string, publishing bool, err error) error {
	var apiErr *apiError
	if !errors.As(err, &apiErr) {
		return fmt.Errorf("bucket %s in %s: %w", bucket, owner, err)
	}
	switch {
	case apiErr.Status == http.StatusNotFound && apiErr.Code == "BUCKET_NOT_FOUND":
		return fmt.Errorf("no bucket %s in workspace %s (`bex s3 ls` lists them): %w", bucket, owner, err)
	case apiErr.Status == http.StatusForbidden && publishing:
		return fmt.Errorf("publishing a bucket needs the workspace's admin role and your own login (an API key or an S3 access key cannot): %w", err)
	}
	return fmt.Errorf("bucket %s in %s: %w", bucket, owner, err)
}
