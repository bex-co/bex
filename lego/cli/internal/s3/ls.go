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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"
)

// listTime is how a listing shows a time: UTC to the second.
const listTime = "2006-01-02 15:04:05"

// pageEach lists bucket under prefix and calls fn with each page of at most
// 1,000 keys, in key order. A delimiter folds what is below it into common
// prefixes.
func pageEach(ctx context.Context, client *sdk.Client, bucket, prefix, delimiter string, fn func(*sdk.ListObjectsV2Output) error) error {
	in := &sdk.ListObjectsV2Input{Bucket: aws.String(bucket)}
	if prefix != "" {
		in.Prefix = aws.String(prefix)
	}
	if delimiter != "" {
		in.Delimiter = aws.String(delimiter)
	}
	pages := sdk.NewListObjectsV2Paginator(client, in)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return err
		}
		if err := fn(page); err != nil {
			return err
		}
	}
	return nil
}

type lsBucket struct {
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt,omitempty"`
}

type lsObject struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	LastModified string `json:"lastModified"`
	ETag         string `json:"etag,omitempty"`
}

type lsOutput struct {
	Bucket         string     `json:"bucket"`
	Prefix         string     `json:"prefix,omitempty"`
	CommonPrefixes []string   `json:"commonPrefixes"`
	Objects        []lsObject `json:"objects"`
	TotalObjects   int64      `json:"totalObjects"`
	TotalSize      int64      `json:"totalSize"`
}

func lsCommand(d deps, g *globals) *cobra.Command {
	var recursive, human, summarize bool
	c := &cobra.Command{
		Use:   "ls [s3://bucket[/prefix]]",
		Short: "List your buckets, or the objects under a prefix",
		Long: "With no argument, lists the workspace's buckets, the system buckets of Render's\n" +
			"`ea objects` included. With a bucket and a prefix, lists one level: a `PRE name/`\n" +
			"line for each folder, then the date, size and name of each object. --recursive\n" +
			"lists every key under the prefix, with its full key.\n\n" +
			"A prefix matches by its characters, so `s3://photos/20` lists `2023/` and `2024/`.\n" +
			"Listing counts as egress, as it does on S3.",
		Example: "  bex s3 ls\n  bex s3 ls s3://photos/2024/ --human-readable --summarize\n  bex s3 ls s3://photos --recursive -o json",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			var loc location
			if len(args) == 1 {
				var err error
				if loc, err = parseRemote(args[0]); err != nil {
					return err
				}
			}
			r, err := d.open(c, g, defaultConcurrency, true)
			if err != nil {
				return err
			}
			defer r.close()
			if len(args) == 0 {
				return r.fail(r.listBuckets())
			}
			return r.fail(r.listObjects(loc, recursive, human, summarize))
		},
	}
	c.Flags().BoolVar(&recursive, "recursive", false, "list every key under the prefix, not one level")
	c.Flags().BoolVar(&human, "human-readable", false, "show sizes in KiB, MiB and GiB")
	c.Flags().BoolVar(&summarize, "summarize", false, "end with the number of objects and their total size")
	return c
}

func (r *run) listBuckets() error {
	out, err := r.client.ListBuckets(r.ctx, &sdk.ListBucketsInput{})
	if err != nil {
		return err
	}
	if r.format != formatText {
		buckets := []lsBucket{}
		for _, b := range out.Buckets {
			buckets = append(buckets, lsBucket{Name: aws.ToString(b.Name), CreatedAt: rfc3339(b.CreationDate)})
		}
		return writeValue(r.out, r.format, map[string]any{"buckets": buckets})
	}
	for _, b := range out.Buckets {
		_, _ = fmt.Fprintf(r.out, "%s %s\n", clock(b.CreationDate), aws.ToString(b.Name))
	}
	return nil
}

func (r *run) listObjects(loc location, recursive, human, summarize bool) error {
	delimiter := "/"
	if recursive {
		delimiter = ""
	}
	// What a line shows of a key: with a delimiter, the name below the folder
	// the prefix names; a recursive listing shows the whole key.
	trim := ""
	if !recursive {
		trim = loc.Key[:strings.LastIndex(loc.Key, "/")+1]
	}
	size := func(n int64) string {
		if human {
			return humanBytes(n)
		}
		return fmt.Sprintf("%d", n)
	}
	doc := lsOutput{Bucket: loc.Bucket, Prefix: loc.Key, CommonPrefixes: []string{}, Objects: []lsObject{}}
	err := pageEach(r.ctx, r.client, loc.Bucket, loc.Key, delimiter, func(page *sdk.ListObjectsV2Output) error {
		for _, p := range page.CommonPrefixes {
			name := aws.ToString(p.Prefix)
			doc.CommonPrefixes = append(doc.CommonPrefixes, name)
			if r.format == formatText {
				_, _ = fmt.Fprintf(r.out, "%27sPRE %s\n", "", strings.TrimPrefix(name, trim))
			}
		}
		for _, o := range page.Contents {
			key, n := aws.ToString(o.Key), aws.ToInt64(o.Size)
			doc.TotalObjects++
			doc.TotalSize += n
			if r.format == formatText {
				_, _ = fmt.Fprintf(r.out, "%s %10s %s\n", clock(o.LastModified), size(n), strings.TrimPrefix(key, trim))
			} else {
				doc.Objects = append(doc.Objects, lsObject{Key: key, Size: n, LastModified: rfc3339(o.LastModified), ETag: strings.Trim(aws.ToString(o.ETag), `"`)})
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if r.format != formatText {
		return writeValue(r.out, r.format, doc)
	}
	if summarize {
		_, _ = fmt.Fprintf(r.out, "\nTotal Objects: %d\n   Total Size: %s\n", doc.TotalObjects, size(doc.TotalSize))
	}
	return nil
}

// clock is a time as a listing shows it, blank when the server sent none.
func clock(t *time.Time) string {
	if t == nil {
		return strings.Repeat(" ", len(listTime))
	}
	return t.UTC().Format(listTime)
}

// rfc3339 is a time as data, "" when absent.
func rfc3339(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
