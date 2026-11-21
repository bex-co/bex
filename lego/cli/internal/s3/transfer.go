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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	// basePartSize is the part size of a multipart transfer. It grows when an
	// object would need more than maxParts of it.
	basePartSize = 8 << 20
	maxParts     = 10000
	// maxObject is the largest object, and the largest copy, the gateway takes
	// (ADR112 D3).
	maxObject = 5 << 30
)

// partSizeFor is the part size for an object of size bytes: 8 MiB, grown in
// whole MiB until the object fits in 10,000 parts. A negative size is a stream
// of unknown length and keeps the base.
func partSizeFor(size int64) int64 {
	if size <= basePartSize*maxParts {
		return basePartSize
	}
	need := (size + maxParts - 1) / maxParts
	const mib = 1 << 20
	return (need + mib - 1) / mib * mib
}

// putOptions are the headers an upload or a copy is asked to set.
type putOptions struct {
	contentType string
	metadata    map[string]string
}

// replaces is whether a copy was asked to set headers of its own.
func (o putOptions) replaces() bool { return o.contentType != "" || len(o.metadata) > 0 }

// parseMetadata reads repeated --metadata key=value flags.
func parseMetadata(pairs []string) (map[string]string, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		k, v, ok := strings.Cut(pair, "=")
		if !ok || k == "" {
			return nil, usageError("--metadata takes key=value, not %q", pair)
		}
		out[k] = v
	}
	return out, nil
}

// guessType is the content type a file's name suggests, "" when it suggests none.
func guessType(name string) string { return mime.TypeByExtension(filepath.Ext(name)) }

// countingFile reports the bytes the uploader reads from a file.
type countingFile struct {
	*os.File
	prog *progress
}

func (f countingFile) Read(p []byte) (int, error) {
	n, err := f.File.Read(p)
	f.prog.advance(int64(n))
	return n, err
}

func (f countingFile) ReadAt(p []byte, off int64) (int, error) {
	n, err := f.File.ReadAt(p, off)
	f.prog.advance(int64(n))
	return n, err
}

// upload sends body as bucket/key, in 8 MiB parts and at most --concurrency
// requests at a time when it is larger than one. size is the length when it is
// known and -1 for a stream.
func (r *run) upload(ctx context.Context, body io.Reader, size int64, bucket, key string, o putOptions) error {
	in := &sdk.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: body}
	if o.contentType != "" {
		in.ContentType = aws.String(o.contentType)
	}
	if len(o.metadata) > 0 {
		in.Metadata = o.metadata
	}
	return r.uploadParts(ctx, in, partSizeFor(size))
}

// uploadFile sends one file.
func (r *run) uploadFile(ctx context.Context, path string, size int64, bucket, key string, o putOptions) error {
	if size > maxObject {
		return fmt.Errorf("%s is %s: an object is at most %s on bex object storage", path, humanBytes(size), humanBytes(maxObject))
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if o.contentType == "" {
		o.contentType = guessType(path)
	}
	return r.upload(ctx, countingFile{File: f, prog: r.prog}, size, bucket, key, o)
}

// countingWriterAt reports the bytes the downloader writes into a file.
type countingWriterAt struct {
	*os.File
	prog *progress
}

func (w countingWriterAt) WriteAt(p []byte, off int64) (int, error) {
	n, err := w.File.WriteAt(p, off)
	w.prog.advance(int64(n))
	return n, err
}

// download fetches bucket/key into dest, through a temporary file beside it
// that is renamed over dest only when every byte arrived, so a failed or
// interrupted download leaves no half file and does not replace an older one.
// size is the object's length and modified the time to give the file.
func (r *run) download(ctx context.Context, bucket, key, dest string, size int64, modified time.Time) (err error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp, err := createBeside(dest)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	if size != 0 {
		n, derr := r.downloadParts(ctx, countingWriterAt{File: tmp, prog: r.prog}, &sdk.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		if derr != nil {
			return derr
		}
		if size > 0 && n != size {
			return fmt.Errorf("s3://%s/%s was %d bytes when listed and %d when downloaded: it changed during the transfer", bucket, key, size, n)
		}
	}
	if !modified.IsZero() {
		if err := os.Chtimes(tmp.Name(), modified, modified); err != nil {
			return err
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}

// createBeside makes a new file next to dest, named for it, with the
// permissions a new file gets.
func createBeside(dest string) (*os.File, error) {
	for range 8 {
		var salt [4]byte
		if _, err := rand.Read(salt[:]); err != nil {
			return nil, err
		}
		name := filepath.Join(filepath.Dir(dest), "."+filepath.Base(dest)+".bex-"+hex.EncodeToString(salt[:]))
		f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o666)
		if !errors.Is(err, fs.ErrExist) {
			return f, err
		}
	}
	return nil, fmt.Errorf("could not make a temporary file beside %s", dest)
}

// streamTo writes bucket/key to w, in order and without a temporary file.
func (r *run) streamTo(ctx context.Context, w io.Writer, bucket, key string) error {
	out, err := r.client.GetObject(ctx, &sdk.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return err
	}
	defer func() { _ = out.Body.Close() }()
	_, err = io.Copy(w, out.Body)
	return err
}

// copySource is the x-amz-copy-source value of an object: bucket and key, with
// each path segment percent-encoded.
func copySource(bucket, key string) string {
	parts := strings.Split(key, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return bucket + "/" + strings.Join(parts, "/")
}

// remoteInfo is what a listing or a HEAD says of an object.
type remoteInfo struct {
	Size        int64
	Modified    time.Time
	ContentType string
}

// infoOf is what a listing says of an object.
func infoOf(o types.Object) remoteInfo {
	info := remoteInfo{Size: aws.ToInt64(o.Size)}
	if o.LastModified != nil {
		info.Modified = *o.LastModified
	}
	return info
}

// isMissingObject is the answer for a key that is not there: a HEAD carries no
// body, so it has only the status to go on.
func isMissingObject(err error) bool {
	var missing *types.NotFound
	f, ok := faultOf(err)
	return errors.As(err, &missing) || (ok && (f.Code == "NotFound" || f.Code == "NoSuchKey"))
}

// head asks for an object's size, time and type. A missing object is named.
func (r *run) head(ctx context.Context, bucket, key string) (remoteInfo, error) {
	out, err := r.client.HeadObject(ctx, &sdk.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		if isMissingObject(err) {
			return remoteInfo{}, fmt.Errorf("there is no object s3://%s/%s", bucket, key)
		}
		return remoteInfo{}, err
	}
	info := remoteInfo{Size: aws.ToInt64(out.ContentLength), ContentType: aws.ToString(out.ContentType)}
	if out.LastModified != nil {
		info.Modified = *out.LastModified
	}
	return info, nil
}

// copyObject copies one object inside the bucket namespace of the workspace
// with a single server-side request. The gateway copies within one workspace
// and one region and answers the rest itself; a copy over 5 GiB cannot exist,
// since no object is that large, and is refused before any request.
func (r *run) copyObject(ctx context.Context, from, to location, info remoteInfo, o putOptions) error {
	if info.Size > maxObject {
		return fmt.Errorf("%s is %s: a copy is at most %s on bex object storage", from, humanBytes(info.Size), humanBytes(maxObject))
	}
	in := &sdk.CopyObjectInput{
		Bucket: aws.String(to.Bucket), Key: aws.String(to.Key), CopySource: aws.String(copySource(from.Bucket, from.Key)),
	}
	if o.replaces() {
		in.MetadataDirective = types.MetadataDirectiveReplace
		in.ContentType = aws.String(info.ContentType)
		if o.contentType != "" {
			in.ContentType = aws.String(o.contentType)
		}
		in.Metadata = o.metadata
	}
	_, err := r.client.CopyObject(ctx, in)
	return err
}

// localFile is a regular file found under a directory.
type localFile struct {
	Rel      string // path below the root, slash separated: the object key below a prefix
	Path     string
	Size     int64
	Modified time.Time
}

// walkLocal calls fn for every regular file under root, in the order the keys
// they become sort in (a directory sorts as its name and a slash), so a listing
// of the same tree merges against it in one pass. Anything that is not a
// regular file or a directory is reported to warn and skipped. A directory it
// cannot read stops the walk: skipping it would make its files look deleted.
func walkLocal(ctx context.Context, root string, warn func(string), fn func(localFile) error) error {
	var walk func(dir, rel string) error
	walk = func(dir, rel string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		sortKey := func(e fs.DirEntry) string {
			if e.IsDir() {
				return e.Name() + "/"
			}
			return e.Name()
		}
		slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(sortKey(a), sortKey(b)) })
		for _, e := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			path, name := filepath.Join(dir, e.Name()), rel+e.Name()
			switch {
			case e.IsDir():
				if err := walk(path, name+"/"); err != nil {
					return err
				}
			case e.Type().IsRegular():
				info, err := e.Info()
				if err != nil {
					return err
				}
				if err := fn(localFile{Rel: name, Path: path, Size: info.Size(), Modified: info.ModTime()}); err != nil {
					return err
				}
			default:
				warn(fmt.Sprintf("warning: skipping %s: it is %s, and bex s3 copies regular files only", path, kindOf(e.Type())))
			}
		}
		return nil
	}
	return walk(root, "")
}

// kindOf names a file type that is not a regular file.
func kindOf(m fs.FileMode) string {
	switch {
	case m&fs.ModeSymlink != 0:
		return "a symbolic link"
	case m&fs.ModeDevice != 0:
		return "a device"
	case m&fs.ModeSocket != 0:
		return "a socket"
	case m&fs.ModeNamedPipe != 0:
		return "a named pipe"
	}
	return "not a regular file"
}

// safeRel reports whether a key's path below its prefix can be a local path
// under the destination directory: no empty, `.` or `..` element, so no object
// can write outside the directory the person named.
func safeRel(rel string) bool {
	if rel == "" || strings.ContainsRune(rel, 0) || strings.HasPrefix(rel, "/") {
		return false
	}
	for _, part := range strings.Split(strings.TrimSuffix(rel, "/"), "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}
