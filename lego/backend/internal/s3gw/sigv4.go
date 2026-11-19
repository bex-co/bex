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

package s3gw

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SigV4 (ADR112 D3): the Authorization header form and the presigned query form,
// verified over the canonical request with the access key's secret. The region
// and service come from the credential scope as the client sent them (the service
// must be s3, the region is compared with nothing), the clock may differ by 15
// minutes, signatures are compared in constant time, and every x-amz-* header and
// host must be signed, in both forms, as S3 requires.

const (
	algorithm         = "AWS4-HMAC-SHA256"
	maxClockSkew      = 15 * time.Minute
	maxPresignExpires = 7 * 24 * time.Hour
	amzDateLayout     = "20060102T150405Z"
	dateLayout        = "20060102"
	unsignedPayload   = "UNSIGNED-PAYLOAD"
)

// authForm is how a request carries its signature.
type authForm int

const (
	formAnonymous authForm = iota
	formHeader
	formPresigned
)

// parsedAuth is a request's signature, parsed and checked for shape but not yet
// verified.
type parsedAuth struct {
	form          authForm
	keyID         string
	date          string // yyyymmdd of the credential scope
	region        string
	signedHeaders []string // lowercase, as sent
	signature     string   // hex
	amzDate       string   // the request time, yyyymmddThhmmssZ
	requestTime   time.Time
	expires       time.Duration // presigned only
	// payloadHash is the hashed-payload line of the canonical request: the
	// x-amz-content-sha256 header of a header-signed request, UNSIGNED-PAYLOAD
	// for a presigned one.
	payloadHash string
}

// classify says how r is signed. SigV2 and a request carrying both mechanisms are
// refused here, before any lookup.
func classify(r *http.Request) (authForm, *Error) {
	header := r.Header.Get("Authorization")
	q := r.URL.Query()
	presigned := q.Has("X-Amz-Algorithm") || q.Has("X-Amz-Signature") || q.Has("X-Amz-Credential")
	switch {
	case header != "" && presigned:
		return formAnonymous, errInvalidArgument("Only one auth mechanism allowed; only the X-Amz-Algorithm query parameter, Signature query string parameter or the Authorization header should be specified")
	case header != "":
		if strings.HasPrefix(header, algorithm+" ") {
			return formHeader, nil
		}
		if strings.HasPrefix(header, "AWS ") {
			return formAnonymous, errNotImplemented("Signature Version 2 is not supported; sign requests with Signature Version 4")
		}
		return formAnonymous, errAuthorizationHeaderMalformed("The authorization header is malformed; the algorithm must be " + algorithm + ".")
	case presigned:
		return formPresigned, nil
	case q.Has("AWSAccessKeyId") || q.Has("Signature"):
		return formAnonymous, errNotImplemented("Signature Version 2 is not supported; sign requests with Signature Version 4")
	}
	return formAnonymous, nil
}

// parseAuth reads the signature out of r. It does the checks that cost nothing
// and need no key: the shape of the fields, the scope's service and terminator,
// the request time against the clock and a presigned URL's lifetime.
func parseAuth(r *http.Request, form authForm, now time.Time) (*parsedAuth, *Error) {
	var p parsedAuth
	p.form = form
	var credential, signedHeaders string
	malformed := errAuthorizationHeaderMalformed
	if form == formPresigned {
		malformed = errAuthorizationQueryParametersError
		q := r.URL.Query()
		if q.Get("X-Amz-Algorithm") != algorithm {
			return nil, malformed("X-Amz-Algorithm must be " + algorithm + ".")
		}
		if q.Has("X-Amz-Security-Token") {
			return nil, errNotImplemented("Session tokens are not supported; use an access key")
		}
		credential, signedHeaders = q.Get("X-Amz-Credential"), q.Get("X-Amz-SignedHeaders")
		p.signature, p.amzDate = q.Get("X-Amz-Signature"), q.Get("X-Amz-Date")
		secs, err := strconv.ParseInt(q.Get("X-Amz-Expires"), 10, 64)
		if err != nil || secs < 1 {
			return nil, malformed("X-Amz-Expires must be a number of seconds, at least 1.")
		}
		if time.Duration(secs)*time.Second > maxPresignExpires {
			return nil, malformed("X-Amz-Expires must be less than a week (in seconds); that is, the given X-Amz-Expires must be less than 604800 seconds.")
		}
		p.expires = time.Duration(secs) * time.Second
		p.payloadHash = unsignedPayload
	} else {
		fields, ok := parseAuthorizationFields(strings.TrimPrefix(r.Header.Get("Authorization"), algorithm+" "))
		if !ok {
			return nil, malformed("The authorization header is malformed; the Credential, SignedHeaders and Signature fields are required.")
		}
		credential, signedHeaders, p.signature = fields["Credential"], fields["SignedHeaders"], fields["Signature"]
		p.amzDate = r.Header.Get("X-Amz-Date")
		if p.amzDate == "" {
			// Without x-amz-date the Date header is the request time, which the
			// client must then have signed.
			t, err := http.ParseTime(r.Header.Get("Date"))
			if err != nil {
				return nil, malformed("The request carries neither a valid x-amz-date nor a valid Date header.")
			}
			p.amzDate = t.UTC().Format(amzDateLayout)
		}
		p.payloadHash = r.Header.Get("X-Amz-Content-Sha256")
		if p.payloadHash == "" {
			return nil, errInvalidRequest("Missing required header for this request: x-amz-content-sha256")
		}
	}
	parts := strings.Split(credential, "/")
	if len(parts) != 5 || parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] != "s3" || parts[4] != "aws4_request" {
		return nil, malformed("The credential must be <access key id>/<date>/<region>/s3/aws4_request.")
	}
	p.keyID, p.date, p.region = parts[0], parts[1], parts[2]
	if len(p.signature) != 64 || !isHex(p.signature) {
		return nil, malformed("The signature must be 64 hexadecimal characters.")
	}
	t, err := time.Parse(amzDateLayout, p.amzDate)
	if err != nil || p.amzDate[:8] != p.date {
		return nil, malformed("The request date must be yyyymmddThhmmssZ, on the date of the credential scope.")
	}
	p.requestTime = t
	p.signedHeaders = splitSignedHeaders(signedHeaders)
	if len(p.signedHeaders) == 0 {
		return nil, malformed("SignedHeaders is required.")
	}
	if form == formPresigned {
		if now.Add(maxClockSkew).Before(t) {
			return nil, errRequestTimeTooSkewed()
		}
		if now.After(t.Add(p.expires)) {
			return nil, errAccessDenied("Request has expired")
		}
	} else if d := now.Sub(t); d > maxClockSkew || -d > maxClockSkew {
		return nil, errRequestTimeTooSkewed()
	}
	return &p, nil
}

// parseAuthorizationFields splits `Credential=..., SignedHeaders=..., Signature=...`.
func parseAuthorizationFields(s string) (map[string]string, bool) {
	fields := map[string]string{}
	for _, part := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || k == "" || v == "" {
			return nil, false
		}
		fields[k] = v
	}
	for _, need := range []string{"Credential", "SignedHeaders", "Signature"} {
		if fields[need] == "" {
			return nil, false
		}
	}
	return fields, true
}

func splitSignedHeaders(s string) []string {
	if s == "" {
		return nil
	}
	names := strings.Split(strings.ToLower(s), ";")
	for _, n := range names {
		if n == "" {
			return nil
		}
	}
	return names
}

func isHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// checkSignedHeaders enforces S3's rule that host and every x-amz-* header be
// signed. A presigned URL is a bearer credential: an unsigned x-amz-copy-source
// added by its holder would otherwise copy any object of the workspace into a
// public key.
func (p *parsedAuth) checkSignedHeaders(r *http.Request) *Error {
	signed := make(map[string]bool, len(p.signedHeaders))
	for _, n := range p.signedHeaders {
		signed[n] = true
	}
	if !signed["host"] {
		return errAccessDenied("There were headers present in the request which were not signed: host")
	}
	var unsigned []string
	for name := range r.Header {
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "x-amz-") && !signed[lower] {
			unsigned = append(unsigned, lower)
		}
	}
	if len(unsigned) > 0 {
		sort.Strings(unsigned)
		return errAccessDenied("There were headers present in the request which were not signed: " + strings.Join(unsigned, ","))
	}
	return nil
}

// verify checks the signature against secret, in constant time.
func (p *parsedAuth) verify(r *http.Request, secret string) *Error {
	canonical := canonicalRequest(r, p)
	sts := algorithm + "\n" + p.amzDate + "\n" + p.date + "/" + p.region + "/s3/aws4_request\n" + hexSHA256([]byte(canonical))
	want := hmacHex(signingKey(secret, p.date, p.region), sts)
	if !hmac.Equal([]byte(want), []byte(p.signature)) {
		return errSignatureDoesNotMatch()
	}
	return nil
}

// canonicalRequest is S3's: the method, the path encoded once, the sorted query
// without the signature, the signed headers and the payload hash.
func canonicalRequest(r *http.Request, p *parsedAuth) string {
	var b strings.Builder
	b.WriteString(r.Method)
	b.WriteByte('\n')
	b.WriteString(uriEncode(r.URL.Path, false))
	b.WriteByte('\n')
	b.WriteString(canonicalQuery(r.URL.RawQuery, p.form == formPresigned))
	b.WriteByte('\n')
	for _, name := range p.signedHeaders {
		b.WriteString(name)
		b.WriteByte(':')
		b.WriteString(canonicalHeaderValue(r, name))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(strings.Join(p.signedHeaders, ";"))
	b.WriteByte('\n')
	b.WriteString(p.payloadHash)
	return b.String()
}

// canonicalHeaderValue is a header's values trimmed, with runs of spaces folded
// to one and several values joined by a comma. host is the request's Host, which
// net/http keeps out of the header map.
func canonicalHeaderValue(r *http.Request, name string) string {
	var values []string
	switch name {
	case "host":
		values = []string{r.Host}
	case "content-length":
		values = r.Header.Values("Content-Length")
		if len(values) == 0 && r.ContentLength >= 0 {
			values = []string{strconv.FormatInt(r.ContentLength, 10)}
		}
	default:
		values = r.Header.Values(name)
	}
	folded := make([]string, len(values)) // never edit the request's own slice
	for i, v := range values {
		folded[i] = strings.Join(strings.Fields(v), " ")
	}
	return strings.Join(folded, ",")
}

type queryPair struct{ k, v string }

// canonicalQuery sorts the query by encoded name and then value, each encoded
// with S3's rule; a parameter with no value is `name=`. The signature of a
// presigned URL is not part of what it signs.
func canonicalQuery(raw string, presigned bool) string {
	var pairs []queryPair
	for _, part := range strings.Split(raw, "&") {
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		key, err1 := url.QueryUnescape(k)
		val, err2 := url.QueryUnescape(v)
		if err1 != nil || err2 != nil {
			key, val = k, v
		}
		if presigned && key == "X-Amz-Signature" {
			continue
		}
		pairs = append(pairs, queryPair{uriEncode(key, true), uriEncode(val, true)})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].k != pairs[j].k {
			return pairs[i].k < pairs[j].k
		}
		return pairs[i].v < pairs[j].v
	})
	var b strings.Builder
	for i, p := range pairs {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(p.k)
		b.WriteByte('=')
		b.WriteString(p.v)
	}
	return b.String()
}

// uriEncode is SigV4's encoding: every byte but A-Z a-z 0-9 - _ . ~ is %XX in
// uppercase, and `/` too unless the value is a path.
func uriEncode(s string, encodeSlash bool) string {
	const hexUpper = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		case c == '/' && !encodeSlash:
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hexUpper[c>>4])
			b.WriteByte(hexUpper[c&15])
		}
	}
	return b.String()
}

func hexSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}

func hmacHex(key []byte, data string) string { return hex.EncodeToString(hmacSHA256(key, data)) }

// signingKey is the derived key of a request's scope.
func signingKey(secret, date, region string) []byte {
	k := hmacSHA256([]byte("AWS4"+secret), date)
	k = hmacSHA256(k, region)
	k = hmacSHA256(k, "s3")
	return hmacSHA256(k, "aws4_request")
}
