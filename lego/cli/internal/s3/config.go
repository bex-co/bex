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
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/spf13/cobra"

	"github.com/bex-co/bex/lego/cli/internal/usage"
)

// maxRulesFile bounds a rules file: the gateway keeps at most 64 KiB of CORS
// rules and 1,000 lifecycle rules, so a larger file is a mistake.
const maxRulesFile = 1 << 20

// corsRule and corsDoc are a bucket's CORS rules in the S3 API's JSON shape,
// the document `get` prints and `set` reads.
type corsRule struct {
	ID             string   `json:"ID,omitempty"`
	AllowedOrigins []string `json:"AllowedOrigins"`
	AllowedMethods []string `json:"AllowedMethods"`
	AllowedHeaders []string `json:"AllowedHeaders,omitempty"`
	ExposeHeaders  []string `json:"ExposeHeaders,omitempty"`
	MaxAgeSeconds  *int32   `json:"MaxAgeSeconds,omitempty"`
}

type corsDoc struct {
	CORSRules []corsRule `json:"CORSRules"`
}

// lifecycleRule and lifecycleDoc are a bucket's lifecycle rules in the S3 API's
// JSON shape. They hold what the gateway stores (ADR112 D3): a prefix filter, a
// status, an expiry by days or date, and the abort of unfinished uploads.
type lifecycleRule struct {
	ID                             string               `json:"ID,omitempty"`
	Status                         string               `json:"Status"`
	Prefix                         *string              `json:"Prefix,omitempty"`
	Filter                         *lifecycleFilter     `json:"Filter,omitempty"`
	Expiration                     *lifecycleExpiration `json:"Expiration,omitempty"`
	AbortIncompleteMultipartUpload *lifecycleAbort      `json:"AbortIncompleteMultipartUpload,omitempty"`
}

type lifecycleFilter struct {
	Prefix *string `json:"Prefix,omitempty"`
}

// lifecycleExpiration's Date is text, not a time, so that set can take a bare
// date as well as RFC 3339 and refuse anything else in a sentence
// (parseLifecycleDate).
type lifecycleExpiration struct {
	Days *int32  `json:"Days,omitempty"`
	Date *string `json:"Date,omitempty"`
}

type lifecycleAbort struct {
	DaysAfterInitiation *int32 `json:"DaysAfterInitiation,omitempty"`
}

type lifecycleDoc struct {
	Rules []lifecycleRule `json:"Rules"`
}

// unknownField picks the field out of the decoder's refusal of one.
var unknownField = regexp.MustCompile(`unknown field "([^"]*)"`)

// decodeDoc reads a JSON document into v, naming the line of the first thing
// wrong with it. A field bex does not store is refused with what is stored.
func decodeDoc(name string, data []byte, v any) error {
	// The decoder would read bytes that are not UTF-8 as U+FFFD, and the
	// rules sent would then differ from the file.
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 {
			return fmt.Errorf("%s: line %d: the file is not UTF-8", name, 1+bytes.Count(data[:i], []byte("\n")))
		}
		i += size
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	err := dec.Decode(v)
	if err == nil && dec.More() {
		return fmt.Errorf("%s: line %d: more than one JSON document", name, 1+bytes.Count(data[:min(dec.InputOffset(), int64(len(data)))], []byte("\n")))
	}
	if err == nil {
		return nil
	}
	offset := dec.InputOffset()
	var syntax *json.SyntaxError
	var typed *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntax):
		offset = syntax.Offset
	case errors.As(err, &typed):
		offset = typed.Offset
	default:
		// The decoder reports an unknown field after reading the whole value, so
		// the field's own name is the better place to point to.
		if m := unknownField.FindStringSubmatch(err.Error()); m != nil {
			if i := bytes.Index(data, []byte(`"`+m[1]+`"`)); i >= 0 {
				offset = int64(i)
			}
		}
	}
	line := 1 + bytes.Count(data[:min(max(offset, 0), int64(len(data)))], []byte("\n"))
	return fmt.Errorf("%s: line %d: %w", name, line, err)
}

// readRules reads the file of a `set`: a path, or `-` for standard input.
func (d deps) readRules(path string) ([]byte, error) {
	in := d.stdin
	name := "standard input"
	if path != stdio {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()
		in, name = f, path
	}
	data, err := io.ReadAll(io.LimitReader(in, maxRulesFile+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxRulesFile {
		return nil, fmt.Errorf("%s is larger than %d MiB: it is not a rules file", name, maxRulesFile>>20)
	}
	return data, nil
}

func corsCommand(d deps, g *globals) *cobra.Command {
	cors := &cobra.Command{
		Use:   "cors",
		Short: "Read, set or delete a bucket's CORS rules",
		Long: "A bucket's CORS rules say which web pages may read it from a browser. `set` reads\n" +
			"a JSON file of the S3 API's shape and replaces the bucket's rules with it:\n\n" +
			`  {"CORSRules": [{"AllowedOrigins": ["https://example.com"], "AllowedMethods": ["GET"],` + "\n" +
			`                  "AllowedHeaders": ["*"], "MaxAgeSeconds": 600}]}` + "\n\n" +
			"A bucket stores up to 100 rules in 64 KiB. Rules apply to the public host of a\n" +
			"bucket (`bex s3 public`), where browsers read it.",
		Example:      "  bex s3 cors set photos --file cors.json\n  bex s3 cors get photos\n  bex s3 cors delete photos",
		SilenceUsage: true,
		Args:         usage.SubcommandArgs,
		RunE:         func(c *cobra.Command, _ []string) error { return c.Help() },
	}
	cors.AddCommand(corsGet(d, g), configSet(d, g, corsRules), configDelete(d, g, corsRules))
	return cors
}

func lifecycleCommand(d deps, g *globals) *cobra.Command {
	lifecycle := &cobra.Command{
		Use:   "lifecycle",
		Short: "Read, set or delete a bucket's lifecycle (expiry) rules",
		Long: "Lifecycle rules delete objects when they age, and abort multipart uploads that\n" +
			"were never finished. `set` reads a JSON file of the S3 API's shape and replaces\n" +
			"the bucket's rules with it:\n\n" +
			`  {"Rules": [{"ID": "logs", "Status": "Enabled", "Filter": {"Prefix": "logs/"},` + "\n" +
			`              "Expiration": {"Days": 7},` + "\n" +
			`              "AbortIncompleteMultipartUpload": {"DaysAfterInitiation": 3}}]}` + "\n\n" +
			"bex runs the rules hourly. It stores a prefix filter, a status, an expiry of 1 to\n" +
			"36,500 days or a date (2027-01-01 is midnight UTC), and the abort of unfinished\n" +
			"uploads, up to 1,000 rules; transitions, versions and the tag and size filters\n" +
			"are not stored.",
		Example:      "  bex s3 lifecycle set photos --file lifecycle.json\n  bex s3 lifecycle get photos -o json\n  bex s3 lifecycle delete photos",
		SilenceUsage: true,
		Args:         usage.SubcommandArgs,
		RunE:         func(c *cobra.Command, _ []string) error { return c.Help() },
	}
	lifecycle.AddCommand(lifecycleGet(d, g), configSet(d, g, lifecycleRules), configDelete(d, g, lifecycleRules))
	return lifecycle
}

// ---- get ----------------------------------------------------------------------

func corsGet(d deps, g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "get <bucket>",
		Short:   "Print a bucket's CORS rules",
		Example: "  bex s3 cors get photos\n  bex s3 cors get photos -o json > cors.json",
		Args:    cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			bucket, err := bucketArg(args[0])
			if err != nil {
				return err
			}
			r, err := d.open(c, g, defaultConcurrency, true)
			if err != nil {
				return err
			}
			defer r.close()
			doc := corsDoc{CORSRules: []corsRule{}}
			out, err := r.client.GetBucketCors(r.ctx, &sdk.GetBucketCorsInput{Bucket: aws.String(bucket)})
			if err != nil && !missingConfig(err, "NoSuchCORSConfiguration") {
				return bucketFault(bucket, r.fail(err))
			}
			if out != nil {
				for _, rule := range out.CORSRules {
					doc.CORSRules = append(doc.CORSRules, corsRule{
						ID: aws.ToString(rule.ID), AllowedOrigins: rule.AllowedOrigins, AllowedMethods: rule.AllowedMethods,
						AllowedHeaders: rule.AllowedHeaders, ExposeHeaders: rule.ExposeHeaders, MaxAgeSeconds: rule.MaxAgeSeconds,
					})
				}
			}
			if r.format != formatText {
				return writeValue(r.out, r.format, doc)
			}
			if len(doc.CORSRules) == 0 {
				_, _ = fmt.Fprintln(r.out, "no CORS rules")
				return nil
			}
			t := newTable(r.out)
			_, _ = fmt.Fprintln(t, "ID\tORIGINS\tMETHODS\tHEADERS\tEXPOSE\tMAX AGE")
			for _, rule := range doc.CORSRules {
				age := ""
				if rule.MaxAgeSeconds != nil {
					age = fmt.Sprintf("%ds", *rule.MaxAgeSeconds)
				}
				_, _ = fmt.Fprintf(t, "%s\t%s\t%s\t%s\t%s\t%s\n", rule.ID, strings.Join(rule.AllowedOrigins, ","), strings.Join(rule.AllowedMethods, ","),
					strings.Join(rule.AllowedHeaders, ","), strings.Join(rule.ExposeHeaders, ","), age)
			}
			return t.Flush()
		},
	}
}

func lifecycleGet(d deps, g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "get <bucket>",
		Short:   "Print a bucket's lifecycle rules",
		Example: "  bex s3 lifecycle get photos\n  bex s3 lifecycle get photos -o json > lifecycle.json",
		Args:    cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			bucket, err := bucketArg(args[0])
			if err != nil {
				return err
			}
			r, err := d.open(c, g, defaultConcurrency, true)
			if err != nil {
				return err
			}
			defer r.close()
			doc := lifecycleDoc{Rules: []lifecycleRule{}}
			out, err := r.client.GetBucketLifecycleConfiguration(r.ctx, &sdk.GetBucketLifecycleConfigurationInput{Bucket: aws.String(bucket)})
			if err != nil && !missingConfig(err, "NoSuchLifecycleConfiguration") {
				return bucketFault(bucket, r.fail(err))
			}
			if out != nil {
				for _, rule := range out.Rules {
					doc.Rules = append(doc.Rules, lifecycleRuleOf(rule))
				}
			}
			if r.format != formatText {
				return writeValue(r.out, r.format, doc)
			}
			if len(doc.Rules) == 0 {
				_, _ = fmt.Fprintln(r.out, "no lifecycle rules")
				return nil
			}
			t := newTable(r.out)
			_, _ = fmt.Fprintln(t, "ID\tSTATUS\tPREFIX\tEXPIRES\tABORT UPLOADS")
			for _, rule := range doc.Rules {
				prefix := ""
				switch {
				case rule.Filter != nil && rule.Filter.Prefix != nil:
					prefix = *rule.Filter.Prefix
				case rule.Prefix != nil:
					prefix = *rule.Prefix
				}
				expires, abort := "", ""
				if e := rule.Expiration; e != nil && e.Days != nil {
					expires = afterDays(*e.Days)
				} else if e != nil && e.Date != nil {
					day, _, _ := strings.Cut(*e.Date, "T") // RFC 3339 in UTC (lifecycleRuleOf)
					expires = "on " + day
				}
				if a := rule.AbortIncompleteMultipartUpload; a != nil && a.DaysAfterInitiation != nil {
					abort = afterDays(*a.DaysAfterInitiation)
				}
				_, _ = fmt.Fprintf(t, "%s\t%s\t%s\t%s\t%s\n", rule.ID, rule.Status, prefix, expires, abort)
			}
			return t.Flush()
		},
	}
}

// lifecycleRuleOf is a stored rule in the document's shape.
func lifecycleRuleOf(rule types.LifecycleRule) lifecycleRule {
	out := lifecycleRule{ID: aws.ToString(rule.ID), Status: string(rule.Status), Prefix: rule.Prefix} //nolint:staticcheck // the gateway stores a legacy prefix as written (ADR112 D3), and get prints it back
	if rule.Filter != nil && rule.Filter.Prefix != nil {
		out.Filter = &lifecycleFilter{Prefix: rule.Filter.Prefix}
	}
	if e := rule.Expiration; e != nil && (e.Days != nil || e.Date != nil) {
		out.Expiration = &lifecycleExpiration{Days: e.Days}
		if e.Date != nil {
			out.Expiration.Date = aws.String(e.Date.UTC().Format(time.RFC3339))
		}
	}
	if a := rule.AbortIncompleteMultipartUpload; a != nil && a.DaysAfterInitiation != nil {
		out.AbortIncompleteMultipartUpload = &lifecycleAbort{DaysAfterInitiation: a.DaysAfterInitiation}
	}
	return out
}

// missingConfig is the answer for a bucket that has no rules of the kind.
func missingConfig(err error, code string) bool {
	f, ok := faultOf(err)
	return ok && f.Code == code
}

// bucketFault names the bucket a refused call was about.
func bucketFault(bucket string, err error) error {
	if f, ok := faultOf(err); ok && f.Code == "NoSuchBucket" {
		return &explained{msg: fmt.Sprintf("there is no bucket %s in this workspace (`bex s3 ls` lists them)", bucket), err: err}
	}
	return err
}

// ---- set and delete -------------------------------------------------------------

// ruleSet is one kind of bucket rules: how to read its file and how to send,
// read or delete it.
type ruleSet struct {
	kind  string // as named to a person: "CORS", "lifecycle"
	verb  string // the command: "cors", "lifecycle"
	parse func(name string, data []byte) (count int, send func(r *run, bucket string) error, err error)
	del   func(r *run, bucket string) error
}

var corsRules = ruleSet{kind: "CORS", verb: "cors", parse: parseCORS, del: func(r *run, bucket string) error {
	_, err := r.client.DeleteBucketCors(r.ctx, &sdk.DeleteBucketCorsInput{Bucket: aws.String(bucket)})
	return err
}}

var lifecycleRules = ruleSet{kind: "lifecycle", verb: "lifecycle", parse: parseLifecycle, del: func(r *run, bucket string) error {
	_, err := r.client.DeleteBucketLifecycle(r.ctx, &sdk.DeleteBucketLifecycleInput{Bucket: aws.String(bucket)})
	return err
}}

// parseCORS checks a CORS document and returns what sends it.
func parseCORS(name string, data []byte) (int, func(*run, string) error, error) {
	var doc corsDoc
	if err := decodeDoc(name, data, &doc); err != nil {
		return 0, nil, err
	}
	if len(doc.CORSRules) == 0 {
		return 0, nil, usageError("%s: line 1: the document has no CORSRules (a document is {\"CORSRules\": [ … ]}); `bex s3 cors delete` removes the rules", name)
	}
	rules := make([]types.CORSRule, 0, len(doc.CORSRules))
	for _, rule := range doc.CORSRules {
		out := types.CORSRule{AllowedOrigins: rule.AllowedOrigins, AllowedMethods: rule.AllowedMethods, AllowedHeaders: rule.AllowedHeaders, ExposeHeaders: rule.ExposeHeaders, MaxAgeSeconds: rule.MaxAgeSeconds}
		if rule.ID != "" {
			out.ID = aws.String(rule.ID)
		}
		rules = append(rules, out)
	}
	return len(rules), func(r *run, bucket string) error {
		_, err := r.client.PutBucketCors(r.ctx, &sdk.PutBucketCorsInput{Bucket: aws.String(bucket), CORSConfiguration: &types.CORSConfiguration{CORSRules: rules}})
		return err
	}, nil
}

// parseLifecycle checks a lifecycle document and returns what sends it.
func parseLifecycle(name string, data []byte) (int, func(*run, string) error, error) {
	var doc lifecycleDoc
	if err := decodeDoc(name, data, &doc); err != nil {
		return 0, nil, err
	}
	if len(doc.Rules) == 0 {
		return 0, nil, usageError("%s: line 1: the document has no Rules (a document is {\"Rules\": [ … ]}); `bex s3 lifecycle delete` removes the rules", name)
	}
	rules := make([]types.LifecycleRule, 0, len(doc.Rules))
	for i, rule := range doc.Rules {
		// A refusal names the rule by position, as the gateway does, and by ID.
		refuse := func(format string, args ...any) error {
			which := fmt.Sprintf("lifecycle rule %d", i+1)
			if rule.ID != "" {
				which += fmt.Sprintf(" (%q)", rule.ID)
			}
			return usageError("%s: %s: %s", name, which, fmt.Sprintf(format, args...))
		}
		out := types.LifecycleRule{Status: types.ExpirationStatus(rule.Status), Prefix: rule.Prefix} //nolint:staticcheck // the gateway takes the legacy prefix as well as Filter.Prefix (ADR112 D3)
		if rule.ID != "" {
			out.ID = aws.String(rule.ID)
		}
		if rule.Filter != nil && rule.Filter.Prefix != nil {
			out.Filter = &types.LifecycleRuleFilter{Prefix: rule.Filter.Prefix}
			if problem := prefixProblem(*rule.Filter.Prefix); problem != "" {
				return 0, nil, refuse("Filter.Prefix %q %s (%s)", *rule.Filter.Prefix, problem, prefixRule)
			}
		}
		if rule.Prefix != nil {
			if problem := prefixProblem(*rule.Prefix); problem != "" {
				return 0, nil, refuse("Prefix %q %s (%s)", *rule.Prefix, problem, prefixRule)
			}
		}
		if e := rule.Expiration; e != nil {
			out.Expiration = &types.LifecycleExpiration{Days: e.Days}
			if e.Date != nil {
				date, ok := parseLifecycleDate(*e.Date)
				if !ok {
					return 0, nil, refuse("Expiration.Date %q must be a date such as 2027-01-01 or 2027-01-01T00:00:00Z", *e.Date)
				}
				out.Expiration.Date = &date
			}
		}
		if a := rule.AbortIncompleteMultipartUpload; a != nil {
			out.AbortIncompleteMultipartUpload = &types.AbortIncompleteMultipartUpload{DaysAfterInitiation: a.DaysAfterInitiation}
		}
		rules = append(rules, out)
	}
	return len(rules), func(r *run, bucket string) error {
		_, err := r.client.PutBucketLifecycleConfiguration(r.ctx, &sdk.PutBucketLifecycleConfigurationInput{
			Bucket: aws.String(bucket), LifecycleConfiguration: &types.BucketLifecycleConfiguration{Rules: rules},
		})
		return err
	}, nil
}

// prefixRule is the gateway's rule for a lifecycle prefix in words.
const prefixRule = "a lifecycle prefix is at most 1,024 bytes with no control character"

// prefixProblem says what breaks the gateway's rule for a lifecycle prefix
// (objectstorage.ValidateLifecycle in lego/backend, which this module cannot
// import), or "" when nothing does. It is checked before the rule is sent
// because the SDK's XML encoder replaces most control characters with U+FFFD
// rather than refuse them, and the gateway would store a prefix that real keys
// match in place of the one the file names (w32/067).
func prefixProblem(prefix string) string {
	switch {
	case len(prefix) > maxKeyBytes:
		return fmt.Sprintf("is %d bytes long", len(prefix))
	case strings.ContainsFunc(prefix, unicode.IsControl):
		return "has a control character"
	}
	return ""
}

// parseLifecycleDate reads an Expiration's Date: RFC 3339, as S3's JSON writes
// it, or a bare date, which is midnight UTC as bex stores it.
func parseLifecycleDate(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339, time.DateOnly} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// afterDays is a rule's age in the lifecycle table.
func afterDays(n int32) string {
	return "after " + counted(int(n), "day")
}

// counted is n of a noun, singular for one: "1 rule", "2 rules".
func counted(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// configSet is `set <bucket> --file rules.json` for one kind of rules.
func configSet(d deps, g *globals, rs ruleSet) *cobra.Command {
	var file string
	c := &cobra.Command{
		Use:     "set <bucket> --file rules.json",
		Short:   "Replace a bucket's " + rs.kind + " rules with those of a JSON file",
		Long:    "Reads the S3 API's JSON document for the rules from --file (`-` is standard input)\nand replaces the bucket's " + rs.kind + " rules with it. `get -o json` prints a document `set` accepts.",
		Example: "  bex s3 " + rs.verb + " set photos --file rules.json",
		Args:    cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			bucket, err := bucketArg(args[0])
			if err != nil {
				return err
			}
			if file == "" {
				return usageError("--file names the JSON document of the rules (`-` for standard input)")
			}
			data, err := d.readRules(file)
			if err != nil {
				return err
			}
			// The document is checked before a key is made for it.
			count, send, err := rs.parse(file, data)
			if err != nil {
				return err
			}
			r, err := d.open(c, g, defaultConcurrency, true)
			if err != nil {
				return err
			}
			defer r.close()
			if err := send(r, bucket); err != nil {
				return bucketFault(bucket, r.fail(err))
			}
			r.rep.add(result{Action: "put_" + rs.verb, Target: bucket + " (" + counted(count, "rule") + ")"})
			return r.rep.finish()
		},
	}
	c.Flags().StringVar(&file, "file", "", "the JSON document of the rules (`-` for standard input)")
	return c
}

// configDelete is `delete <bucket>` for one kind of rules.
func configDelete(d deps, g *globals, rs ruleSet) *cobra.Command {
	return &cobra.Command{
		Use:     "delete <bucket>",
		Short:   "Delete all of a bucket's " + rs.kind + " rules",
		Example: "  bex s3 " + rs.verb + " delete photos",
		Args:    cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			bucket, err := bucketArg(args[0])
			if err != nil {
				return err
			}
			r, err := d.open(c, g, defaultConcurrency, true)
			if err != nil {
				return err
			}
			defer r.close()
			if err := rs.del(r, bucket); err != nil {
				return bucketFault(bucket, r.fail(err))
			}
			r.rep.add(result{Action: "delete_" + rs.verb, Target: bucket})
			return r.rep.finish()
		},
	}
}
