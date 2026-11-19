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
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"net/http"
	"strings"
)

// Error is an S3 error answer: the status, S3's published code and a message.
// Errors are XML with an x-amz-request-id header, and SDKs retry a 5xx and a
// SlowDown but not a 4xx, so the code and the status have to be right.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// errorBody is the XML document S3 answers with.
type errorBody struct {
	XMLName   xml.Name `xml:"Error"`
	Code      string   `xml:"Code"`
	Message   string   `xml:"Message"`
	Resource  string   `xml:"Resource,omitempty"`
	RequestID string   `xml:"RequestId"`
}

func newError(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// The codes bex answers with (ADR112 D3). A code bex invents says so in its
// name: QuotaExceeded is bex's own.
func errAccessDenied(msg string) *Error { return newError(http.StatusForbidden, "AccessDenied", msg) }
func errInvalidAccessKeyID() *Error {
	return newError(http.StatusForbidden, "InvalidAccessKeyId", "The access key Id you provided does not exist in our records.")
}
func errSignatureDoesNotMatch() *Error {
	return newError(http.StatusForbidden, "SignatureDoesNotMatch", "The request signature we calculated does not match the signature you provided. Check your key and signing method.")
}
func errRequestTimeTooSkewed() *Error {
	return newError(http.StatusForbidden, "RequestTimeTooSkewed", "The difference between the request time and the current time is too large.")
}
func errNotImplemented(msg string) *Error {
	return newError(http.StatusNotImplemented, "NotImplemented", msg)
}
func errSlowDown(msg string) *Error { return newError(http.StatusServiceUnavailable, "SlowDown", msg) }
func errServiceUnavailable(msg string) *Error {
	return newError(http.StatusServiceUnavailable, "ServiceUnavailable", msg)
}
func errInvalidArgument(msg string) *Error {
	return newError(http.StatusBadRequest, "InvalidArgument", msg)
}
func errMalformedXML(msg string) *Error {
	return newError(http.StatusBadRequest, "MalformedXML", msg)
}
func errInvalidRequest(msg string) *Error {
	return newError(http.StatusBadRequest, "InvalidRequest", msg)
}
func errAuthorizationHeaderMalformed(msg string) *Error {
	return newError(http.StatusBadRequest, "AuthorizationHeaderMalformed", msg)
}
func errAuthorizationQueryParametersError(msg string) *Error {
	return newError(http.StatusBadRequest, "AuthorizationQueryParametersError", msg)
}
func errInternal() *Error {
	return newError(http.StatusInternalServerError, "InternalError", "We encountered an internal error. Please try again.")
}

// requestID is 16 uppercase hex characters, the shape of S3's.
func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "0000000000000000"
	}
	return strings.ToUpper(hex.EncodeToString(b[:]))
}

// writeError answers err. The request id is the one the entry point put on the
// response, so a client's report names the request the log has.
func writeError(w http.ResponseWriter, r *http.Request, e *Error) {
	body := errorBody{Code: e.Code, Message: e.Message, Resource: r.URL.Path, RequestID: w.Header().Get("x-amz-request-id")}
	out, err := xml.Marshal(body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	// An answer that carries a body the client did not finish sending closes the
	// connection: the unread body must not be parsed as the next request.
	if r.ContentLength != 0 && r.Body != nil && r.Body != http.NoBody {
		w.Header().Set("Connection", "close")
	}
	w.WriteHeader(e.Status)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(out)
}
