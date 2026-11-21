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
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
)

const urlScheme = "s3://"

// location is one side of a copy: a place in a bucket or a path on this
// machine ("-" is standard input or output).
type location struct {
	Remote bool
	Bucket string
	// Key is the object key or, for a listing or a recursive command, the
	// prefix.
	Key string
	// Path is the local path when the location is not remote.
	Path string
}

// parseLocation reads an argument as `s3://bucket[/key]` or a local path.
func parseLocation(arg string) (location, error) {
	if arg == "" {
		return location{}, usageError("an empty path")
	}
	if !strings.HasPrefix(arg, urlScheme) {
		return location{Path: arg}, nil
	}
	rest := strings.TrimPrefix(arg, urlScheme)
	bucket, key, _ := strings.Cut(rest, "/")
	if !bucketName.MatchString(bucket) {
		return location{}, usageError("%q does not name a bucket: write s3://bucket/key, with a bucket of 3 to 63 lowercase letters, digits and hyphens", arg)
	}
	return location{Remote: true, Bucket: bucket, Key: key}, nil
}

// parseRemote is parseLocation for an argument that must be in a bucket.
func parseRemote(arg string) (location, error) {
	loc, err := parseLocation(arg)
	if err != nil {
		return location{}, err
	}
	if !loc.Remote {
		return location{}, usageError("%q is a local path: name a bucket location as s3://bucket/key", arg)
	}
	return loc, nil
}

// String is the location as the person typed it.
func (l location) String() string {
	switch {
	case !l.Remote:
		return l.Path
	case l.Key == "":
		return urlScheme + l.Bucket
	}
	return urlScheme + l.Bucket + "/" + l.Key
}

// asDir treats a key as a directory's prefix: recursive commands copy and
// delete what is inside `photos/`, not what merely starts with `photos`.
func asDir(prefix string) string {
	if prefix == "" || strings.HasSuffix(prefix, "/") {
		return prefix
	}
	return prefix + "/"
}

// joinKey is prefix and a slash-separated relative path as one key.
func joinKey(prefix, rel string) string { return asDir(prefix) + rel }

// destinationKey is the key a single object lands on: the given key, or, when
// it names a directory (empty or ending in a slash), that directory with the
// source's own name.
func destinationKey(key, sourceName string) string {
	if key == "" || strings.HasSuffix(key, "/") {
		return key + sourceName
	}
	return key
}

// baseName is the last element of a key or a slash-separated path.
func baseName(key string) string { return path.Base(strings.TrimRight(key, "/")) }

// filterRule is one --include or --exclude.
type filterRule struct {
	include bool
	pattern string
	re      *regexp.Regexp
}

// filters apply in the order they were given and the last rule that matches a
// path decides; a path no rule matches is included. A pattern is a glob over
// the path relative to the source directory or prefix, where * also matches
// slashes.
type filters []filterRule

// filterFlag is --include or --exclude, both appending to one list so that
// their relative order on the command line is kept.
type filterFlag struct {
	include bool
	rules   *filters
}

func (f filterFlag) String() string { return "" }
func (f filterFlag) Type() string   { return "pattern" }

func (f filterFlag) Set(pattern string) error {
	re, err := globRegexp(pattern)
	if err != nil {
		return err
	}
	*f.rules = append(*f.rules, filterRule{include: f.include, pattern: pattern, re: re})
	return nil
}

// allows reports whether rel passes the filters.
func (fs filters) allows(rel string) bool {
	allowed := true
	for _, rule := range fs {
		if rule.re.MatchString(rel) {
			allowed = rule.include
		}
	}
	return allowed
}

// globRegexp compiles a glob: * any run of characters, ? one character, [abc]
// and [!abc] a class.
func globRegexp(pattern string) (*regexp.Regexp, error) {
	runes := []rune(pattern)
	var b strings.Builder
	b.WriteString("(?s)^")
	for i := 0; i < len(runes); i++ {
		switch c := runes[i]; c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '[':
			end := slices.Index(runes[i+1:], ']')
			if end < 0 {
				return nil, fmt.Errorf("the pattern %q has a [ with no closing ]", pattern)
			}
			class := string(runes[i+1 : i+1+end])
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			b.WriteString("[" + strings.ReplaceAll(class, `\`, `\\`) + "]")
			i += end + 1
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil, fmt.Errorf("the pattern %q is not a glob: %w", pattern, err)
	}
	return re, nil
}
