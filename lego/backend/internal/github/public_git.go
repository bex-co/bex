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

package github

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/netutil"
)

const (
	publicGitAdvertisementType = "application/x-git-upload-pack-advertisement"
	maxPublicGitAdvertisement  = 1 << 20
	maxPublicGitPacket         = 65520
	maxPublicGitPackets        = 16384
)

var anonymousPublicGitClient = &http.Client{
	Timeout: publicCommitResolveTimeout,
	Transport: &http.Transport{
		Proxy:                  nil,
		DialContext:            netutil.SafeDialContext(publicCommitResolveTimeout),
		ForceAttemptHTTP2:      true,
		TLSHandshakeTimeout:    publicCommitResolveTimeout,
		ResponseHeaderTimeout:  publicCommitResolveTimeout,
		MaxResponseHeaderBytes: 64 << 10,
		MaxIdleConns:           16,
		MaxIdleConnsPerHost:    2,
		IdleConnTimeout:        90 * time.Second,
		DisableCompression:     true,
	},
	CheckRedirect: publicGitRedirect,
}

// resolvePublicGitCommit reads only the anonymous upload-pack advertisement:
// no GitHub token, Git subprocess, credential helper, packfile or object fetch.
// Failure is a cached miss like resolvePublicCommit, so provenance never stops
// a deploy or distinguishes private, missing, malformed and unreachable repos.
func (s *Service) resolvePublicGitCommit(ctx context.Context, repoURL, ref string) (store.CommitInfo, bool, error) {
	endpoint, ok := publicGitAdvertisementURL(repoURL)
	if !ok || !validPublicGitRef(ref) {
		return store.CommitInfo{}, false, nil
	}
	key := "git-http:" + endpoint + "\x00" + ref
	if cached, hit := s.publicCommitMemo().Get(key); hit {
		return cached.info, cached.ok, nil
	}
	value, _, _ := s.publicCommitFlight.Do(key, func() (any, error) {
		if cached, hit := s.publicCommitMemo().Get(key); hit {
			return cached, nil
		}
		ctx, cancel := context.WithTimeout(ctx, publicCommitResolveTimeout)
		defer cancel()
		entry := cachedPublicCommit{}
		ttl := publicCommitMissTTL
		if hash := s.readPublicGitCommit(ctx, endpoint, ref); hash != "" {
			entry = cachedPublicCommit{info: store.CommitInfo{Hash: hash}, ok: true}
			ttl = publicCommitCacheTTL
		}
		s.publicCommitMemo().Put(key, entry, time.Now().Add(ttl))
		return entry, nil
	})
	entry, _ := value.(cachedPublicCommit)
	return entry.info, entry.ok, nil
}

func publicGitDestination(u *url.URL) bool {
	if u == nil || u.Scheme != "https" || u.Host == "" || u.Hostname() == "" || u.User != nil || u.Opaque != "" || u.Fragment != "" || strings.Contains(u.Hostname(), "%") {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil && netutil.UnsafeOriginIP(ip) {
		return false
	}
	if port := u.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return false
		}
	} else if strings.HasSuffix(u.Host, ":") {
		return false
	}
	return true
}

func publicGitAdvertisementURL(raw string) (string, bool) {
	if len(raw) > 4096 {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || !publicGitDestination(u) || u.RawQuery != "" || u.ForceQuery {
		return "", false
	}
	path := strings.TrimRight(u.Path, "/")
	if path == "" || strings.Contains(path, "\\") {
		return "", false
	}
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if part == "" || part == "." || part == ".." {
			return "", false
		}
	}
	// Preserve escaped path components (and case) rather than using the
	// GitHub-oriented CanonicalRepo, which lowercases repository paths.
	escaped := strings.TrimRight(u.EscapedPath(), "/")
	if strings.HasSuffix(strings.ToLower(escaped), "%2f") {
		return "", false
	}
	u.Scheme = "https"
	u.Host = strings.ToLower(u.Host)
	u.Path, u.RawPath = path+"/info/refs", escaped+"/info/refs"
	u.RawQuery = "service=git-upload-pack"
	return u.String(), true
}

func publicGitRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > 3 || !publicGitDestination(req.URL) {
		return errors.New("public Git redirect is not permitted")
	}
	req.Header.Del("Authorization")
	req.Header.Del("Proxy-Authorization")
	req.Header.Del("Cookie")
	return nil
}

func (s *Service) readPublicGitCommit(ctx context.Context, endpoint, ref string) string {
	client := anonymousPublicGitClient
	if s.publicGitHTTPClient != nil {
		copy := *s.publicGitHTTPClient
		copy.Timeout = publicCommitResolveTimeout
		copy.CheckRedirect = publicGitRedirect
		copy.Jar = nil
		client = &copy
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", publicGitAdvertisementType)
	// No Git-Protocol header requests protocolv0: its advertisement includes
	// refs directly. Protocolv2 would require a second ls-refs operation.
	response, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = response.Body.Close() }()
	media, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if response.StatusCode != http.StatusOK || err != nil || media != publicGitAdvertisementType {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxPublicGitAdvertisement+1))
	if err != nil || len(data) > maxPublicGitAdvertisement {
		return ""
	}
	refs, head, ok := parsePublicGitAdvertisement(data)
	if !ok {
		return ""
	}
	return publicGitAdvertisedCommit(refs, head, ref)
}

// publicGitPacket enforces Git's four-hex-digit, length-includes-header framing.
// See https://git-scm.com/docs/http-protocol#_smart_server_response and
// https://git-scm.com/docs/protocol-common#_pkt_line_format .
func publicGitPacket(data []byte) (payload, rest []byte, flush, ok bool) {
	if len(data) < 4 {
		return nil, nil, false, false
	}
	for _, c := range data[:4] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return nil, nil, false, false
		}
	}
	size, err := strconv.ParseUint(string(data[:4]), 16, 16)
	if err != nil {
		return nil, nil, false, false
	}
	if size == 0 {
		return nil, data[4:], true, true
	}
	if size < 4 || size > maxPublicGitPacket || int(size) > len(data) {
		return nil, nil, false, false
	}
	return data[4:size], data[size:], false, true
}

func parsePublicGitAdvertisement(data []byte) (map[string]string, string, bool) {
	header, data, flush, ok := publicGitPacket(data)
	if !ok || flush || string(bytes.TrimSuffix(header, []byte{'\n'})) != "# service=git-upload-pack" {
		return nil, "", false
	}
	_, data, flush, ok = publicGitPacket(data)
	if !ok || !flush {
		return nil, "", false
	}
	refs := map[string]string{}
	head := ""
	first := true
	versionSeen := false
	for count := 0; count < maxPublicGitPackets; count++ {
		payload, rest, flush, ok := publicGitPacket(data)
		if !ok {
			return nil, "", false
		}
		data = rest
		if flush {
			return refs, head, len(data) == 0
		}
		line := strings.TrimSuffix(string(payload), "\n")
		if first && line == "version 1" {
			if versionSeen {
				return nil, "", false
			}
			versionSeen = true
			continue
		}
		record, capabilities, hasCaps := strings.Cut(line, "\x00")
		if first != hasCaps {
			return nil, "", false
		}
		hash, name, ok := strings.Cut(record, " ")
		if !ok || !core.ValidCommitSHA(hash) {
			return nil, "", false
		}
		if first {
			for capability := range strings.FieldsSeq(capabilities) {
				if target, ok := strings.CutPrefix(capability, "symref=HEAD:"); ok {
					head = target
				}
			}
		}
		first = false
		if name == "capabilities^{}" && strings.Trim(hash, "0") == "" {
			continue
		}
		plain := strings.TrimSuffix(name, "^{}")
		if plain != name && !strings.HasPrefix(plain, "refs/tags/") {
			return nil, "", false
		}
		if (plain != "HEAD" && !strings.HasPrefix(plain, "refs/")) || !validPublicGitRef(plain) || strings.Trim(hash, "0") == "" {
			return nil, "", false
		}
		hash = strings.ToLower(hash)
		if previous, present := refs[name]; present && previous != hash {
			return nil, "", false
		}
		refs[name] = hash
	}
	return nil, "", false
}

func validPublicGitRef(ref string) bool {
	if ref == "HEAD" || core.ValidCommitSHA(ref) {
		return true
	}
	if ref == "" || len(ref) > 1024 || strings.ContainsAny(ref, " ~^:?*[\\") || strings.Contains(ref, "..") || strings.Contains(ref, "@{") {
		return false
	}
	for _, c := range ref {
		if c < 0x20 || c == 0x7f {
			return false
		}
	}
	for _, part := range strings.Split(ref, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".") || strings.HasSuffix(part, ".lock") {
			return false
		}
	}
	return true
}

// Under Git's v0 advertisement contract, an annotated tag MUST have a
// peeled ^{} entry. A tag without one is treated as lightweight; independently
// checking object types would require fetching Git objects or a packfile.
func publicGitAdvertisedCommit(refs map[string]string, head, ref string) string {
	if core.ValidCommitSHA(ref) {
		ref = strings.ToLower(ref)
		// A custom ref can advertise the same tag object as refs/tags/*.
		// Prefer its known peeled target independently of map iteration order.
		for name, hash := range refs {
			if hash == ref && strings.HasPrefix(name, "refs/tags/") && !strings.HasSuffix(name, "^{}") {
				if peeled := refs[name+"^{}"]; peeled != "" {
					return peeled
				}
			}
		}
		for _, hash := range refs {
			if hash == ref {
				return hash
			}
		}
		return ""
	}
	if ref == "HEAD" {
		if hash := refs["HEAD"]; hash != "" {
			return hash
		}
		return refs[head]
	}
	if strings.HasPrefix(ref, "refs/") {
		if hash := refs[ref+"^{}"]; hash != "" && strings.HasPrefix(ref, "refs/tags/") {
			return hash
		}
		return refs[ref]
	}
	if hash := refs["refs/heads/"+ref]; hash != "" {
		return hash
	}
	tag := "refs/tags/" + ref
	if hash := refs[tag+"^{}"]; hash != "" {
		return hash
	}
	return refs[tag]
}
