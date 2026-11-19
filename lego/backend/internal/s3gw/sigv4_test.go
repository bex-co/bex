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
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

var exampleNow = time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC)

// requestOf rebuilds the request a known answer signed.
func requestOf(t *testing.T, a knownAnswer) *http.Request {
	t.Helper()
	target := a.url
	if a.kind == "presigned" {
		// botocore returns the URL it signed; the signature is what it appends.
		target += "&X-Amz-Signature=" + a.signature
	}
	r, err := http.NewRequest(a.method, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range a.headers {
		if strings.EqualFold(k, "host") {
			r.Host = v
			continue
		}
		r.Header.Set(k, v)
	}
	if a.kind == "header" {
		r.Header.Set("Authorization", fmt.Sprintf("%s Credential=%s/%s/%s/s3/aws4_request, SignedHeaders=%s, Signature=%s",
			algorithm, exampleAccessKey, r.Header.Get("X-Amz-Date")[:8], a.region, a.signedHeaders, a.signature))
	}
	return r
}

// verifyAt runs the verifier the way the gateway does, minus the key lookup.
func verifyAt(r *http.Request, secret string, now time.Time) *Error {
	form, e := classify(r)
	if e != nil {
		return e
	}
	p, e := parseAuth(r, form, now)
	if e != nil {
		return e
	}
	if e := p.checkSignedHeaders(r); e != nil {
		return e
	}
	return p.verify(r, secret)
}

func codeOf(e *Error) string {
	if e == nil {
		return ""
	}
	return e.Code
}

func TestTheVerifierAcceptsEverySignatureBotocoreAndTheDocumentationProduce(t *testing.T) {
	for _, a := range knownAnswers {
		t.Run(a.name, func(t *testing.T) {
			r := requestOf(t, a)
			now := exampleNow
			if a.kind == "presigned" {
				now = exampleNow.Add(time.Minute)
			}
			if e := verifyAt(r, exampleSecretKey, now); e != nil {
				t.Fatalf("verify = %v, want the signature accepted", e)
			}
		})
	}
	if len(knownAnswers) < 8 {
		t.Fatalf("only %d known answers", len(knownAnswers))
	}
}

func TestAnyChangeToASignedRequestIsASignatureMismatch(t *testing.T) {
	for _, a := range knownAnswers {
		mutations := map[string]func(r *http.Request){
			"another secret": nil,
			"another method": func(r *http.Request) {
				r.Method = map[string]string{"GET": "DELETE"}[r.Method] + map[string]string{"PUT": "GET", "POST": "PUT"}[r.Method]
			},
			"another path":    func(r *http.Request) { r.URL.Path += "x" },
			"another query":   func(r *http.Request) { r.URL.RawQuery += "&extra=1" },
			"another host":    func(r *http.Request) { r.Host = "evil.example" },
			"another payload": func(r *http.Request) { r.Header.Set("X-Amz-Content-Sha256", strings.Repeat("a", 64)) },
			"a signed header": func(r *http.Request) {
				r.Header.Set("Range", "bytes=1-2")
				r.Header.Set("x-amz-meta-mtime", "9")
				r.Header.Set("Content-Type", "x/y")
			},
			"another signature": func(r *http.Request) {
				r.Header.Set("Authorization", strings.Replace(r.Header.Get("Authorization"), "Signature=", "Signature=0", 1)[:len(r.Header.Get("Authorization"))])
			},
			"another region key": nil,
		}
		for name, mutate := range mutations {
			if a.kind == "presigned" && (name == "another payload" || name == "a signed header" || name == "another signature") {
				continue
			}
			if name == "another region key" {
				continue
			}
			t.Run(a.name+"/"+name, func(t *testing.T) {
				r := requestOf(t, a)
				secret := exampleSecretKey
				switch name {
				case "another secret":
					secret = exampleSecretKey + "x"
				default:
					mutate(r)
				}
				now := exampleNow
				if a.kind == "presigned" {
					now = exampleNow.Add(time.Minute)
				}
				e := verifyAt(r, secret, now)
				// A mutation that unsigns or adds an x-amz header is refused as
				// such before the signature is even computed.
				if code := codeOf(e); code != "SignatureDoesNotMatch" && code != "AccessDenied" {
					t.Fatalf("verify after %q = %v, want SignatureDoesNotMatch or AccessDenied", name, e)
				}
			})
		}
	}
}

func TestTheClockMayDifferByFifteenMinutes(t *testing.T) {
	a := knownAnswers[0]
	for _, c := range []struct {
		skew time.Duration
		want string
	}{
		{14 * time.Minute, ""}, {-14 * time.Minute, ""},
		{16 * time.Minute, "RequestTimeTooSkewed"}, {-16 * time.Minute, "RequestTimeTooSkewed"}, {48 * time.Hour, "RequestTimeTooSkewed"},
	} {
		if got := codeOf(verifyAt(requestOf(t, a), exampleSecretKey, exampleNow.Add(c.skew))); got != c.want {
			t.Errorf("clock %v off = %q, want %q", c.skew, got, c.want)
		}
	}
}

func TestAPresignedURLExpiresAndIsBoundedToAWeek(t *testing.T) {
	var presigned knownAnswer
	for _, a := range knownAnswers {
		if a.name == "aws-doc-presigned-get" {
			presigned = a
		}
	}
	// X-Amz-Expires is 86400.
	if e := verifyAt(requestOf(t, presigned), exampleSecretKey, exampleNow.Add(86400*time.Second)); e != nil {
		t.Errorf("at the last second: %v", e)
	}
	if got := codeOf(verifyAt(requestOf(t, presigned), exampleSecretKey, exampleNow.Add(86401*time.Second))); got != "AccessDenied" {
		t.Errorf("one second past the expiry = %q, want AccessDenied (Request has expired)", got)
	}
	if got := codeOf(verifyAt(requestOf(t, presigned), exampleSecretKey, exampleNow.Add(-16*time.Minute))); got != "RequestTimeTooSkewed" {
		t.Errorf("a URL dated in the future = %q, want RequestTimeTooSkewed", got)
	}
	over := requestOf(t, presigned)
	q := over.URL.Query()
	q.Set("X-Amz-Expires", "604801")
	over.URL.RawQuery = q.Encode()
	if got := codeOf(verifyAt(over, exampleSecretKey, exampleNow)); got != "AuthorizationQueryParametersError" {
		t.Errorf("X-Amz-Expires over a week = %q, want AuthorizationQueryParametersError", got)
	}
	zero := requestOf(t, presigned)
	q = zero.URL.Query()
	q.Set("X-Amz-Expires", "0")
	zero.URL.RawQuery = q.Encode()
	if got := codeOf(verifyAt(zero, exampleSecretKey, exampleNow)); got != "AuthorizationQueryParametersError" {
		t.Errorf("X-Amz-Expires 0 = %q", got)
	}
}

func TestEveryXAmzHeaderAndHostMustBeSigned(t *testing.T) {
	// A presigned URL is a bearer credential: an unsigned x-amz-copy-source added
	// by its holder would copy any object of the workspace into a public key.
	var presigned, header knownAnswer
	for _, a := range knownAnswers {
		switch a.name {
		case "aws-doc-presigned-get":
			presigned = a
		case "aws-doc-get-range":
			header = a
		}
	}
	for _, a := range []knownAnswer{presigned, header} {
		r := requestOf(t, a)
		r.Header.Set("x-amz-copy-source", "/b/secret")
		now := exampleNow.Add(time.Minute)
		e := verifyAt(r, exampleSecretKey, now)
		if codeOf(e) != "AccessDenied" || !strings.Contains(e.Message, "x-amz-copy-source") {
			t.Errorf("%s with an unsigned x-amz-copy-source = %v, want AccessDenied naming it", a.name, e)
		}
	}
	// host unsigned.
	r := requestOf(t, header)
	r.Header.Set("Authorization", strings.Replace(r.Header.Get("Authorization"), "SignedHeaders=host;", "SignedHeaders=", 1))
	if got := codeOf(verifyAt(r, exampleSecretKey, exampleNow)); got != "AccessDenied" {
		t.Errorf("host not signed = %q, want AccessDenied", got)
	}
}

func TestSigV2AndMixedAuthenticationAreRefusedBeforeAnyLookup(t *testing.T) {
	v2 := func(mutate func(r *http.Request)) string {
		r, _ := http.NewRequest("GET", "https://objects.onbex.co/b/k", nil)
		mutate(r)
		_, e := classify(r)
		return codeOf(e)
	}
	if got := v2(func(r *http.Request) {
		r.Header.Set("Authorization", "AWS AKIAIOSFODNN7EXAMPLE:frJIUN8DYpKDtOLCwo//yllqDzg=")
	}); got != "NotImplemented" {
		t.Errorf("SigV2 header = %q, want NotImplemented", got)
	}
	if got := v2(func(r *http.Request) { r.URL.RawQuery = "AWSAccessKeyId=AKIA&Signature=x&Expires=1" }); got != "NotImplemented" {
		t.Errorf("SigV2 query = %q, want NotImplemented", got)
	}
	if got := v2(func(r *http.Request) {
		r.Header.Set("Authorization", algorithm+" Credential=a/b/c/s3/aws4_request, SignedHeaders=host, Signature=x")
		r.URL.RawQuery = "X-Amz-Algorithm=" + algorithm
	}); got != "InvalidArgument" {
		t.Errorf("two mechanisms = %q, want InvalidArgument", got)
	}
	if got := v2(func(r *http.Request) { r.Header.Set("Authorization", "Bearer abc") }); got != "AuthorizationHeaderMalformed" {
		t.Errorf("a bearer token = %q, want AuthorizationHeaderMalformed", got)
	}
}

func TestMalformedSignaturesAreRefusedWithS3sCodes(t *testing.T) {
	a := knownAnswers[0]
	cases := map[string]func(auth string) string{
		"service not s3":     func(s string) string { return strings.Replace(s, "/s3/aws4_request", "/ec2/aws4_request", 1) },
		"terminator":         func(s string) string { return strings.Replace(s, "aws4_request", "aws5_request", 1) },
		"short signature":    func(s string) string { return strings.Replace(s, "Signature="+a.signature, "Signature=abcd", 1) },
		"upper-case hex":     func(s string) string { return strings.Replace(s, a.signature, strings.ToUpper(a.signature), 1) },
		"missing field":      func(s string) string { return strings.Replace(s, ", SignedHeaders="+a.signedHeaders, "", 1) },
		"date mismatch":      func(s string) string { return strings.Replace(s, "/20130524/", "/20130525/", 1) },
		"empty region":       func(s string) string { return strings.Replace(s, "/us-east-1/", "//", 1) },
		"empty signed names": func(s string) string { return strings.Replace(s, a.signedHeaders, "host;;range", 1) },
	}
	for name, mutate := range cases {
		r := requestOf(t, a)
		r.Header.Set("Authorization", mutate(r.Header.Get("Authorization")))
		if got := codeOf(verifyAt(r, exampleSecretKey, exampleNow)); got != "AuthorizationHeaderMalformed" {
			t.Errorf("%s = %q, want AuthorizationHeaderMalformed", name, got)
		}
	}
	// A request with no payload hash header is a 400, as S3 answers.
	r := requestOf(t, a)
	r.Header.Del("X-Amz-Content-Sha256")
	if e := verifyAt(r, exampleSecretKey, exampleNow); e == nil || e.Status != http.StatusBadRequest {
		t.Errorf("no x-amz-content-sha256 = %v, want a 400", e)
	}
}

func TestTheRegionIsWhateverTheClientSignedAndNothingIsComparedWithIt(t *testing.T) {
	for _, region := range []string{"oregon", "us-east-1", "frankfurt", "eu-central-2", "anything-at-all"} {
		a := knownAnswers[0]
		a.region = region
		// A signature made for another region does not verify under this one,
		// but each region the client names is accepted as its own scope: the
		// bucket's row decides where an object lives (ADR112 D1).
		r := requestOf(t, a)
		if got := codeOf(verifyAt(r, exampleSecretKey, exampleNow)); region != "us-east-1" && got != "SignatureDoesNotMatch" {
			t.Errorf("region %q with a us-east-1 signature = %q, want SignatureDoesNotMatch", region, got)
		}
	}
}

func TestCanonicalEncodingFollowsSigV4(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/photos/a b+c/é.txt", "/photos/a%20b%2Bc/%C3%A9.txt"},
		{"/a/b~c_d-e.f", "/a/b~c_d-e.f"},
		{"/ünï", "/%C3%BCn%C3%AF"},
		{"/a*b(c)", "/a%2Ab%28c%29"},
	}
	for _, c := range cases {
		if got := uriEncode(c.in, false); got != c.want {
			t.Errorf("uriEncode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := uriEncode("a/b", true); got != "a%2Fb" {
		t.Errorf("a slash in a query value = %q, want %%2F", got)
	}
	got := canonicalQuery("prefix=a%2Fb%20c&delimiter=%2F&max-keys=2&uploads&b=2&b=1", false)
	if want := "b=1&b=2&delimiter=%2F&max-keys=2&prefix=a%2Fb%20c&uploads="; got != want {
		t.Errorf("canonicalQuery = %q, want %q", got, want)
	}
	if got := canonicalQuery("a=1&X-Amz-Signature=deadbeef", true); got != "a=1" {
		t.Errorf("a presigned query keeps its signature: %q", got)
	}
	// A plus in a query value is a space, which SigV4 then encodes as %20.
	if got := canonicalQuery("prefix=a+b", false); got != "prefix=a%20b" {
		t.Errorf("canonicalQuery(+) = %q", got)
	}
}

func TestHeaderValuesAreFoldedWithoutEditingTheRequest(t *testing.T) {
	r, _ := http.NewRequest("GET", "https://objects.onbex.co/b", nil)
	r.Header.Add("X-Amz-Meta-Note", "  a   b ")
	r.Header.Add("X-Amz-Meta-Note", "c")
	if got := canonicalHeaderValue(r, "x-amz-meta-note"); got != "a b,c" {
		t.Errorf("canonicalHeaderValue = %q, want %q", got, "a b,c")
	}
	if got := r.Header.Values("X-Amz-Meta-Note")[0]; got != "  a   b " {
		t.Errorf("signing edited the request's own header: %q", got)
	}
	r.Host = "objects.onbex.co:8080"
	if got := canonicalHeaderValue(r, "host"); got != "objects.onbex.co:8080" {
		t.Errorf("host = %q", got)
	}
}
