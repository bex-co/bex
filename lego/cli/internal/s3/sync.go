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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/spf13/cobra"
)

// entry is a file or an object found by a sync: its key below the sync root
// (slash separated, the order both sides sort in), its size and its time.
type entry struct {
	Key      string
	Size     int64
	Modified time.Time
	Path     string // the local path, for a file
}

// entrySource yields the entries of one side in key order.
type entrySource interface {
	next() (e entry, ok bool, err error)
}

// listSource is the objects under a prefix. It holds one page of the listing,
// at most 1,000 keys, and asks for the next only when the merge has used it up.
type listSource struct {
	ctx    context.Context
	pages  *sdk.ListObjectsV2Paginator
	prefix string
	rules  filters
	page   []entry
	pos    int
}

func (r *run) listSource(ctx context.Context, bucket, prefix string, rules filters) *listSource {
	in := &sdk.ListObjectsV2Input{Bucket: aws.String(bucket)}
	if prefix != "" {
		in.Prefix = aws.String(prefix)
	}
	return &listSource{ctx: ctx, pages: sdk.NewListObjectsV2Paginator(r.client, in), prefix: prefix, rules: rules}
}

func (s *listSource) next() (entry, bool, error) {
	for s.pos >= len(s.page) {
		if !s.pages.HasMorePages() {
			return entry{}, false, nil
		}
		out, err := s.pages.NextPage(s.ctx)
		if err != nil {
			return entry{}, false, err
		}
		s.page, s.pos = s.page[:0], 0
		for _, o := range out.Contents {
			key := aws.ToString(o.Key)
			rel := strings.TrimPrefix(key, s.prefix)
			if strings.HasSuffix(key, "/") || !s.rules.allows(rel) {
				continue // a folder marker is not a file, and a filter drops what it excludes on both sides alike
			}
			info := infoOf(o)
			s.page = append(s.page, entry{Key: rel, Size: info.Size, Modified: info.Modified})
		}
	}
	e := s.page[s.pos]
	s.pos++
	return e, true, nil
}

// walkSource is the regular files under a directory, produced by a walk that
// runs one entry ahead of the merge.
type walkSource struct {
	ch <-chan walked
}

type walked struct {
	e   entry
	err error
}

func (r *run) walkSource(ctx context.Context, root string, rules filters) (*walkSource, error) {
	if info, err := os.Stat(root); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &walkSource{ch: closedWalk()}, nil // a directory a download has not made yet is empty
		}
		return nil, err
	} else if !info.IsDir() {
		return nil, usageError("%s is not a directory: sync takes a directory and a prefix", root)
	}
	ch := make(chan walked, 1)
	go func() {
		defer close(ch)
		err := walkLocal(ctx, root, r.warn, func(f localFile) error {
			if !rules.allows(f.Rel) {
				return nil
			}
			select {
			case ch <- walked{e: entry{Key: f.Rel, Size: f.Size, Modified: f.Modified, Path: f.Path}}:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		if err != nil {
			select {
			case ch <- walked{err: err}:
			case <-ctx.Done():
			}
		}
	}()
	return &walkSource{ch: ch}, nil
}

func closedWalk() <-chan walked {
	ch := make(chan walked)
	close(ch)
	return ch
}

func (s *walkSource) next() (entry, bool, error) {
	w, ok := <-s.ch
	if !ok {
		return entry{}, false, nil
	}
	return w.e, w.err == nil, w.err
}

// syncStep is what a merge decides for one key.
type syncStep struct {
	copy bool // else delete the destination's
	src  entry
	dst  entry
}

// modTime is the time a comparison uses for a file or an object: whole seconds,
// which is all an object's time has.
func modTime(t time.Time) time.Time { return t.Truncate(time.Second) }

// needsCopy is the rule of sync: the destination lacks the file, or has one of
// another size, or (unless sizeOnly) one older than the source. Times are
// compared in whole seconds, which is all an object has.
func needsCopy(src, dst entry, sizeOnly bool) bool {
	if src.Size != dst.Size {
		return true
	}
	return !sizeOnly && modTime(src.Modified).After(modTime(dst.Modified))
}

// merge walks two sources in key order, once each, and calls step for every
// key that needs work: a copy of what is missing or changed, and, with
// deleteExtra, a delete of what only the destination has. It holds one entry
// of each side, so its memory is the sources', a page of a listing at most. It
// stops when step returns false.
func merge(src, dst entrySource, sizeOnly, deleteExtra bool, step func(syncStep) bool) error {
	s, sok, err := src.next()
	if err != nil {
		return err
	}
	d, dok, err := dst.next()
	if err != nil {
		return err
	}
	for sok || dok {
		var advanceSrc, advanceDst bool
		switch {
		case !dok || (sok && s.Key < d.Key):
			advanceSrc = true
			if !step(syncStep{copy: true, src: s}) {
				return nil
			}
		case !sok || d.Key < s.Key:
			advanceDst = true
			if deleteExtra && !step(syncStep{dst: d}) {
				return nil
			}
		default:
			advanceSrc, advanceDst = true, true
			if needsCopy(s, d, sizeOnly) && !step(syncStep{copy: true, src: s, dst: d}) {
				return nil
			}
		}
		if advanceSrc {
			if s, sok, err = src.next(); err != nil {
				return err
			}
		}
		if advanceDst {
			if d, dok, err = dst.next(); err != nil {
				return err
			}
		}
	}
	return nil
}

func syncCommand(d deps, g *globals) *cobra.Command {
	var deleteExtra, sizeOnly, yes bool
	o := &transferOptions{recursive: true}
	c := &cobra.Command{
		Use:   "sync <from> <to>",
		Short: "Copy what is missing or changed between a directory and a prefix",
		Long: "Makes the destination hold what the source holds: it copies every file or\n" +
			"object that is missing from the destination, differs in size, or is newer in the\n" +
			"source. Both sides are a directory or a prefix, as a local directory and\n" +
			"s3://bucket/prefix in either order, or two prefixes in one workspace and region.\n" +
			"Run again, it copies nothing. A downloaded file is given its object's time.\n\n" +
			"--delete also deletes what only the destination has; --size-only compares sizes\n" +
			"and ignores times; --include and --exclude pick paths below each root by a glob,\n" +
			"the last matching one deciding, and apply to both sides, so a path they leave out\n" +
			"is never deleted. --dryrun prints the plan and changes nothing. Only regular\n" +
			"files are copied: a symbolic link, a device or a socket is skipped with a warning.\n\n" +
			"--delete asks \"Are you sure?\" in a terminal, as `bex ea objects delete` does;\n" +
			"--yes or --confirm skips the question, and a pipe, a script or CI is never asked.",
		Example: "  bex s3 sync ./site s3://photos/site\n  bex s3 sync s3://photos/site ./site --dryrun\n  bex s3 sync ./site s3://photos/site --delete --exclude '*.map'",
		Args:    cobra.ExactArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			src, err := parseLocation(args[0])
			if err != nil {
				return err
			}
			dst, err := parseLocation(args[1])
			if err != nil {
				return err
			}
			if !src.Remote && !dst.Remote {
				return usageError("one side of sync must be a prefix in a bucket (s3://bucket/prefix)")
			}
			if src.Path == stdio || dst.Path == stdio {
				return usageError("sync takes a directory and a prefix, not standard input or output")
			}
			if err := checkConcurrency(o.concurrency); err != nil {
				return err
			}
			src.Key, dst.Key = asDir(src.Key), asDir(dst.Key)
			if src.Remote && dst.Remote && src.Bucket == dst.Bucket && (strings.HasPrefix(src.Key, dst.Key) || strings.HasPrefix(dst.Key, src.Key)) {
				return usageError("%s and %s overlap: a sync cannot read what it is writing", src, dst)
			}
			if deleteExtra && !o.dryrun && !d.confirmBulkDelete(c, yes, "what is in '"+dst.String()+"' and not in '"+src.String()+"'") {
				return nil
			}
			r, err := d.open(c, g, o.concurrency, o.noProgress)
			if err != nil {
				return err
			}
			defer r.close()
			if err := r.sync(src, dst, o, sizeOnly, deleteExtra); err != nil {
				return err
			}
			return r.rep.finish()
		},
	}
	c.Flags().BoolVar(&deleteExtra, "delete", false, "delete what is in the destination and not in the source")
	c.Flags().BoolVar(&sizeOnly, "size-only", false, "compare sizes only, not times")
	c.Flags().BoolVar(&o.dryrun, "dryrun", false, "show what would be done and do nothing")
	c.Flags().Var(filterFlag{include: false, rules: &o.rules}, "exclude", "leave out paths that match this glob (repeatable)")
	c.Flags().Var(filterFlag{include: true, rules: &o.rules}, "include", "include paths that match this glob, after --exclude (repeatable)")
	c.Flags().BoolVar(&o.noProgress, "no-progress", false, "do not draw the progress line")
	yesFlag(c, &yes)
	concurrencyFlag(c, &o.concurrency)
	return c
}

// sourceOf is the entries of one side.
func (r *run) sourceOf(ctx context.Context, loc location, rules filters) (entrySource, error) {
	if loc.Remote {
		return r.listSource(ctx, loc.Bucket, loc.Key, rules), nil
	}
	return r.walkSource(ctx, loc.Path, rules)
}

// sync merges the two sides and performs each step it decides on.
func (r *run) sync(src, dst location, o *transferOptions, sizeOnly, deleteExtra bool) error {
	w := r.newGroup(o.concurrency)
	from, err := r.sourceOf(w.ctx, src, o.rules)
	if err != nil {
		w.cancel(err)
		return err
	}
	to, err := r.sourceOf(w.ctx, dst, o.rules)
	if err != nil {
		w.cancel(err)
		return err
	}
	var doomed []types.ObjectIdentifier // keys to delete from a bucket, a batch at a time
	flush := func() {
		keys := doomed
		doomed = nil
		if len(keys) > 0 {
			w.do(fmt.Sprintf("delete %d objects of s3://%s", len(keys), dst.Bucket), func(ctx context.Context) error {
				return r.deleteKeys(ctx, dst.Bucket, keys)
			})
		}
	}
	mergeErr := merge(from, to, sizeOnly, deleteExtra, func(s syncStep) bool {
		if !w.alive() {
			return false
		}
		if s.copy {
			j, ok := r.syncJob(src, dst, s.src, o)
			if ok {
				r.execute(w, j)
			}
			return w.alive()
		}
		switch {
		case o.dryrun:
			r.rep.add(result{Action: "delete", Target: dst.entryName(s.dst.Key), DryRun: true})
		case dst.Remote:
			doomed = append(doomed, types.ObjectIdentifier{Key: aws.String(dst.Key + s.dst.Key)})
			if len(doomed) == deleteBatch {
				flush()
			}
		default:
			path := filepath.Join(dst.Path, filepath.FromSlash(s.dst.Key))
			r.execute(w, job{label: "delete " + path, run: func(context.Context) error {
				if err := os.Remove(path); err != nil {
					return err
				}
				r.rep.add(result{Action: "delete", Target: path})
				return nil
			}})
		}
		return w.alive()
	})
	flush()
	return r.finishGroup(w, mergeErr)
}

// entryName is the location of a key below a sync root, as a line prints it.
func (l location) entryName(key string) string {
	if l.Remote {
		return urlScheme + l.Bucket + "/" + l.Key + key
	}
	return filepath.Join(l.Path, filepath.FromSlash(key))
}

// syncJob is the copy of one entry from src's side to dst's.
func (r *run) syncJob(src, dst location, e entry, o *transferOptions) (job, bool) {
	switch {
	case !src.Remote:
		return r.uploadJob(e.Path, e.Size, dst.Bucket, dst.Key+e.Key, o), true
	case !dst.Remote:
		if !safeRel(e.Key) {
			r.warn(fmt.Sprintf("warning: skipping s3://%s/%s%s: its name is not a path that stays inside %s", src.Bucket, src.Key, e.Key, dst.Path))
			return job{}, false
		}
		return r.downloadJob(location{Remote: true, Bucket: src.Bucket, Key: src.Key + e.Key}, dst.entryName(e.Key), remoteInfo{Size: e.Size, Modified: e.Modified}, o), true
	}
	return r.copyJob(location{Remote: true, Bucket: src.Bucket, Key: src.Key + e.Key}, location{Remote: true, Bucket: dst.Bucket, Key: dst.Key + e.Key},
		remoteInfo{Size: e.Size, Modified: e.Modified}, o), true
}
