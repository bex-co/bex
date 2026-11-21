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
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"
)

// stdio is the path that means standard input or output.
const stdio = "-"

// transferOptions are the flags of cp and mv.
type transferOptions struct {
	recursive   bool
	dryrun      bool
	move        bool
	rules       filters
	contentType string
	metadata    []string
	concurrency int
	noProgress  bool

	put putOptions
}

func cpCommand(d deps, g *globals) *cobra.Command { return transferCommand(d, g, false) }
func mvCommand(d deps, g *globals) *cobra.Command { return transferCommand(d, g, true) }

func transferCommand(d deps, g *globals, move bool) *cobra.Command {
	o := &transferOptions{move: move}
	verb, short, long, example := "cp", "Copy a file, a directory or an object", cpHelp, cpExample
	if move {
		verb, short, long, example = "mv", "Move a file, a directory or an object: copy it, then delete the source", mvHelp, mvExample
	}
	c := &cobra.Command{
		Use:     verb + " <from> <to>",
		Short:   short,
		Long:    long,
		Example: example,
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
			if err := o.check(src, dst, verb); err != nil {
				return err
			}
			r, err := d.open(c, g, o.concurrency, o.noProgress)
			if err != nil {
				return err
			}
			defer r.close()
			switch {
			case !src.Remote && dst.Remote:
				err = r.cpUp(src, dst, o)
			case src.Remote && !dst.Remote:
				err = r.cpDown(src, dst, o)
			default:
				err = r.cpRemote(src, dst, o)
			}
			if err != nil {
				return err
			}
			return r.rep.finish()
		},
	}
	c.Flags().BoolVar(&o.recursive, "recursive", false, "copy a directory or a prefix and everything under it")
	c.Flags().BoolVar(&o.dryrun, "dryrun", false, "show what would be done and do nothing")
	c.Flags().Var(filterFlag{include: false, rules: &o.rules}, "exclude", "skip paths that match this glob (repeatable)")
	c.Flags().Var(filterFlag{include: true, rules: &o.rules}, "include", "include paths that match this glob, after --exclude (repeatable)")
	c.Flags().StringVar(&o.contentType, "content-type", "", "the Content-Type to set on what is uploaded or copied (default: guessed from the file name)")
	c.Flags().StringArrayVar(&o.metadata, "metadata", nil, "user metadata key=value to set on what is uploaded or copied (repeatable)")
	c.Flags().BoolVar(&o.noProgress, "no-progress", false, "do not draw the progress line")
	concurrencyFlag(c, &o.concurrency)
	return c
}

// check validates the command line before any key is made.
func (o *transferOptions) check(src, dst location, verb string) error {
	if !src.Remote && !dst.Remote {
		return usageError("one side of %s must be in a bucket (s3://bucket/key): for local files use your shell's own %s", verb, verb)
	}
	if err := checkConcurrency(o.concurrency); err != nil {
		return err
	}
	md, err := parseMetadata(o.metadata)
	if err != nil {
		return err
	}
	o.put = putOptions{contentType: o.contentType, metadata: md}
	if src.Path == stdio || dst.Path == stdio {
		switch {
		case o.recursive:
			return usageError("--recursive copies a directory or a prefix, not standard input or output")
		case o.move:
			return usageError("mv has nothing to remove after standard input or output")
		case src.Path == stdio && (dst.Key == "" || strings.HasSuffix(dst.Key, "/")):
			return usageError("to upload standard input, name the object: s3://%s/key", dst.Bucket)
		case dst.Path == stdio && src.Key == "":
			return usageError("to write an object to standard output, name it: s3://%s/key", src.Bucket)
		}
	}
	if len(o.rules) > 0 && !o.recursive {
		return usageError("--include and --exclude choose among the files of a --recursive transfer")
	}
	return nil
}

// action is the verb a result line starts with.
func (o *transferOptions) action(plain string) string {
	if o.move {
		return "move"
	}
	return plain
}

// job is one file's transfer.
type job struct {
	label string
	size  int64
	run   func(ctx context.Context) error
}

// execute queues a job on the group of a recursive transfer, which reports each
// failure and goes on.
func (r *run) execute(w *group, j job) {
	r.prog.expect(max(j.size, 0))
	w.do(j.label, func(ctx context.Context) error {
		defer r.prog.finished()
		return j.run(ctx)
	})
}

// single runs one job, the only one of a transfer, and returns its error in
// bex's words.
func (r *run) single(j job) error {
	r.prog.expect(max(j.size, 0))
	defer r.prog.finished()
	return r.fail(j.run(r.ctx))
}

// ---- up ---------------------------------------------------------------------

func (r *run) cpUp(src, dst location, o *transferOptions) error {
	if src.Path == stdio {
		return r.single(job{label: "upload -", size: -1, run: func(ctx context.Context) error {
			if !o.dryrun {
				if err := r.upload(ctx, r.d.stdin, -1, dst.Bucket, dst.Key, o.put); err != nil {
					return err
				}
			}
			r.rep.add(result{Action: o.action("upload"), Source: stdio, Destination: dst.String(), DryRun: o.dryrun})
			return nil
		}})
	}
	info, err := os.Stat(src.Path)
	if err != nil {
		return err
	}
	if !o.recursive {
		if info.IsDir() {
			return usageError("%s is a directory: --recursive copies it", src.Path)
		}
		key := destinationKey(dst.Key, filepath.Base(src.Path))
		return r.single(r.uploadJob(src.Path, info.Size(), dst.Bucket, key, o))
	}
	if !info.IsDir() {
		return usageError("%s is not a directory: --recursive copies a directory", src.Path)
	}
	w := r.newGroup(o.concurrency)
	err = walkLocal(w.ctx, src.Path, r.warn, func(f localFile) error {
		if !o.rules.allows(f.Rel) {
			return nil
		}
		r.execute(w, r.uploadJob(f.Path, f.Size, dst.Bucket, joinKey(dst.Key, f.Rel), o))
		return w.ctx.Err()
	})
	return r.finishGroup(w, err)
}

// finishGroup waits for a group and returns the first thing that went wrong:
// what stopped it, else what ended the producer's walk, else its failure count.
func (r *run) finishGroup(w *group, produced error) error {
	if err := w.wait(); err != nil {
		return err
	}
	return r.fail(produced)
}

// uploadJob is the upload of one file, and for mv the removal of the file
// after it arrived.
func (r *run) uploadJob(path string, size int64, bucket, key string, o *transferOptions) job {
	dest := location{Remote: true, Bucket: bucket, Key: key}
	return job{label: "upload " + path, size: size, run: func(ctx context.Context) error {
		if !o.dryrun {
			if err := r.uploadFile(ctx, path, size, bucket, key, o.put); err != nil {
				return err
			}
			if o.move {
				if err := os.Remove(path); err != nil {
					return err
				}
			}
		}
		r.rep.add(result{Action: o.action("upload"), Source: path, Destination: dest.String(), Bytes: size, DryRun: o.dryrun})
		return nil
	}}
}

// warn prints a warning line to stderr.
func (r *run) warn(line string) {
	r.prog.clear()
	_, _ = fmt.Fprintln(r.errOut, line)
}

// ---- down -------------------------------------------------------------------

func (r *run) cpDown(src, dst location, o *transferOptions) error {
	if dst.Path == stdio {
		if o.dryrun {
			r.rep.add(result{Action: "download", Source: src.String(), Destination: stdio, DryRun: true})
			return nil
		}
		return r.fail(r.streamTo(r.ctx, r.out, src.Bucket, src.Key))
	}
	if !o.recursive {
		if err := needObject(src); err != nil {
			return err
		}
		info, err := r.head(r.ctx, src.Bucket, src.Key)
		if err != nil {
			return r.fail(err)
		}
		dest := dst.Path
		if isDir(dest) || strings.HasSuffix(dest, string(os.PathSeparator)) || strings.HasSuffix(dest, "/") {
			dest = filepath.Join(dest, baseName(src.Key))
		}
		return r.single(r.downloadJob(src, dest, info, o))
	}
	if info, err := os.Stat(dst.Path); err == nil && !info.IsDir() {
		return usageError("%s is not a directory: a --recursive download needs one", dst.Path)
	}
	prefix := asDir(src.Key)
	w := r.newGroup(o.concurrency)
	err := pageEach(w.ctx, r.client, src.Bucket, prefix, "", func(page *sdk.ListObjectsV2Output) error {
		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			rel := strings.TrimPrefix(key, prefix)
			if !o.rules.allows(rel) {
				continue
			}
			if !safeRel(rel) {
				r.warn(fmt.Sprintf("warning: skipping s3://%s/%s: its name is not a path that stays inside %s", src.Bucket, key, dst.Path))
				continue
			}
			dest := filepath.Join(dst.Path, filepath.FromSlash(rel))
			if strings.HasSuffix(key, "/") { // a folder marker
				if !o.dryrun {
					if err := os.MkdirAll(dest, 0o755); err != nil {
						return err
					}
				}
				continue
			}
			r.execute(w, r.downloadJob(location{Remote: true, Bucket: src.Bucket, Key: key}, dest, infoOf(obj), o))
			if !w.alive() {
				return w.ctx.Err()
			}
		}
		return nil
	})
	return r.finishGroup(w, err)
}

// downloadJob is the download of one object, and for mv the deletion of the
// object after it arrived.
func (r *run) downloadJob(src location, dest string, info remoteInfo, o *transferOptions) job {
	return job{label: "download " + src.String(), size: info.Size, run: func(ctx context.Context) error {
		if !o.dryrun {
			if err := r.download(ctx, src.Bucket, src.Key, dest, info.Size, info.Modified); err != nil {
				return err
			}
			if o.move {
				if _, err := r.client.DeleteObject(ctx, &sdk.DeleteObjectInput{Bucket: aws.String(src.Bucket), Key: aws.String(src.Key)}); err != nil {
					return err
				}
			}
		}
		r.rep.add(result{Action: o.action("download"), Source: src.String(), Destination: dest, Bytes: info.Size, DryRun: o.dryrun})
		return nil
	}}
}

// needObject refuses a bucket where a single transfer needs an object.
func needObject(loc location) error {
	if loc.Key == "" {
		return usageError("%q names a bucket: name an object (s3://%s/key), or add --recursive for a prefix", loc.String(), loc.Bucket)
	}
	return nil
}

// isDir reports whether path is an existing directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// ---- between buckets ----------------------------------------------------------

func (r *run) cpRemote(src, dst location, o *transferOptions) error {
	if !o.recursive {
		if err := needObject(src); err != nil {
			return err
		}
		to := location{Remote: true, Bucket: dst.Bucket, Key: destinationKey(dst.Key, baseName(src.Key))}
		if src.Bucket == to.Bucket && src.Key == to.Key {
			return usageError("%s and %s are the same object", src, to)
		}
		info, err := r.head(r.ctx, src.Bucket, src.Key)
		if err != nil {
			return r.fail(err)
		}
		return r.single(r.copyJob(src, to, info, o))
	}
	from, to := asDir(src.Key), asDir(dst.Key)
	if src.Bucket == dst.Bucket && (strings.HasPrefix(to, from) || strings.HasPrefix(from, to)) {
		return usageError("%s and %s overlap: a recursive copy or move cannot read what it is writing", src, dst)
	}
	w := r.newGroup(o.concurrency)
	err := pageEach(w.ctx, r.client, src.Bucket, from, "", func(page *sdk.ListObjectsV2Output) error {
		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			rel := strings.TrimPrefix(key, from)
			if !o.rules.allows(rel) {
				continue
			}
			r.execute(w, r.copyJob(location{Remote: true, Bucket: src.Bucket, Key: key}, location{Remote: true, Bucket: dst.Bucket, Key: to + rel}, infoOf(obj), o))
			if !w.alive() {
				return w.ctx.Err()
			}
		}
		return nil
	})
	return r.finishGroup(w, err)
}

// copyJob is the server-side copy of one object, and for mv the deletion of the
// source once the copy succeeded.
func (r *run) copyJob(src, dst location, info remoteInfo, o *transferOptions) job {
	return job{label: "copy " + src.String(), size: 0, run: func(ctx context.Context) error {
		if !o.dryrun {
			if info.ContentType == "" && o.put.replaces() {
				full, err := r.head(ctx, src.Bucket, src.Key)
				if err != nil {
					return err
				}
				info.ContentType = full.ContentType
			}
			if err := r.copyObject(ctx, src, dst, info, o.put); err != nil {
				return err
			}
			if o.move {
				if _, err := r.client.DeleteObject(ctx, &sdk.DeleteObjectInput{Bucket: aws.String(src.Bucket), Key: aws.String(src.Key)}); err != nil {
					return err
				}
			}
		}
		r.rep.add(result{Action: o.action("copy"), Source: src.String(), Destination: dst.String(), Bytes: info.Size, DryRun: o.dryrun})
		return nil
	}}
}

const (
	cpHelp = "Copies a file to a bucket, an object to a file, or an object to another\n" +
		"bucket. The two sides are a local path and an s3:// location, in either order,\n" +
		"or two s3:// locations in one workspace and region (the copy happens on the\n" +
		"server). `-` is standard input or output.\n\n" +
		"Large files go in 8 MiB parts, several at a time (--concurrency, default 8, at\n" +
		"most 10 requests in flight). An object is at most 5 GiB. A download is written\n" +
		"to a temporary file beside its target and renamed when it is whole, so a failed\n" +
		"transfer leaves no partial file. With --recursive, a source ending in no slash is\n" +
		"read as a directory (`s3://photos/2023` copies what is in `2023/`), and --include\n" +
		"and --exclude pick files by a glob over the path below it, the last matching one\n" +
		"deciding. Progress goes to stderr, and only on a terminal. -o json prints the\n" +
		"transfers as data."
	cpExample = "  bex s3 cp cat.jpg s3://photos/cat.jpg\n  bex s3 cp s3://photos/cat.jpg .\n  bex s3 cp ./site s3://photos/site --recursive --exclude '*.map'\n" +
		"  bex s3 cp s3://photos/a.jpg s3://backup/a.jpg\n  tar c . | bex s3 cp - s3://photos/backup.tar"
	mvHelp = "Moves a file, a directory or an object: it is copied, and the source is deleted\n" +
		"only after its copy succeeded, one file at a time. The sides, the flags and the\n" +
		"limits are those of `bex s3 cp`; standard input and output cannot be moved."
	mvExample = "  bex s3 mv ./report.pdf s3://photos/report.pdf\n  bex s3 mv s3://photos/tmp s3://photos/archive --recursive"
)
