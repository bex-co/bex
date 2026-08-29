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

package push

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/hkdf"

	"github.com/bex-co/bex/lego/types/netutil"
)

// Web Push is implemented in-tree (RFC 8291 aes128gcm + RFC 8292 VAPID) with
// stdlib crypto plus the repo's existing jwt and hkdf modules. A third-party
// webpush SDK is deliberately not added: Expo was hand-rolled the same way,
// and an extra HTTP client would bypass the bounded error taxonomy below.

const (
	webPushDefaultTimeout = 10 * time.Second
	webPushTTLSeconds     = 24 * 60 * 60
	webPushMaxBody        = MaxPayloadBytes
	webPushRecordSize     = 4096
	vapidMaxAge           = 12 * time.Hour
)

// WebPushConfig is the complete VAPID composition. All three fields empty is
// the only disabled state and returns (nil, nil) before allocating a client.
// A partial set fails closed at construction so a misconfigured replica cannot
// silently skip browser delivery.
type WebPushConfig struct {
	PublicKey  string
	PrivateKey string
	Subscriber string
	Timeout    time.Duration
}

// WebPush sends one encrypted Web Push request. It is not a push.Transport:
// browsers have no Expo-style receipt poll.
type WebPush struct {
	publicB64  string
	privateKey *ecdsa.PrivateKey
	subscriber string
	http       httpDoer
}

// GenerateVAPIDKeys mints an uncompressed P-256 point and 32-byte scalar as
// unpadded base64url, the encoding browsers and RFC 8292 expect.
func GenerateVAPIDKeys() (publicKey, privateKey string, err error) {
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		base64.RawURLEncoding.EncodeToString(key.Bytes()), nil
}

// NewWebPush constructs the VAPID sender. Empty config returns (nil, nil).
func NewWebPush(config WebPushConfig, options ...Option) (*WebPush, error) {
	pub := strings.TrimSpace(config.PublicKey)
	priv := strings.TrimSpace(config.PrivateKey)
	sub := strings.TrimSpace(config.Subscriber)
	if pub == "" && priv == "" && sub == "" {
		return nil, nil
	}
	if pub == "" || priv == "" || sub == "" {
		return nil, errors.New("push: BEX_WEBPUSH_VAPID_PUBLIC_KEY, BEX_WEBPUSH_VAPID_PRIVATE_KEY, and BEX_WEBPUSH_SUBSCRIBER must all be set")
	}
	if err := validateVAPIDSubscriber(sub); err != nil {
		return nil, err
	}
	privKey, err := parseVAPIDPrivate(priv)
	if err != nil {
		return nil, err
	}
	wantPub, err := decodeKeyMaterial(pub)
	if err != nil || len(wantPub) != 65 || wantPub[0] != 0x04 {
		return nil, errors.New("push: BEX_WEBPUSH_VAPID_PUBLIC_KEY must be an uncompressed P-256 point")
	}
	gotPub := elliptic.Marshal(elliptic.P256(), privKey.PublicKey.X, privKey.PublicKey.Y)
	if !bytes.Equal(gotPub, wantPub) {
		return nil, errors.New("push: BEX_WEBPUSH_VAPID_PUBLIC_KEY does not match the private key")
	}
	opts := expoOptions{}
	for _, option := range options {
		option(&opts)
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = webPushDefaultTimeout
	}
	doer := opts.doer
	if doer == nil {
		// SECURITY (codex round-16 #6): tenant-controlled push endpoints must
		// not reach loopback/private/link-local/metadata via the control-plane
		// pod. Match outbound webhooks: SafeDialContext, no ambient proxy, no
		// redirects.
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.DialContext = netutil.SafeDialContext(timeout)
		tr.Proxy = nil
		doer = &http.Client{
			Timeout:   timeout,
			Transport: tr,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return &WebPush{
		publicB64:  base64.RawURLEncoding.EncodeToString(wantPub),
		privateKey: privKey,
		subscriber: sub,
		http:       doer,
	}, nil
}

// PublicKey is the uncompressed P-256 point in unpadded base64url, the value
// browsers pass as applicationServerKey.
func (w *WebPush) PublicKey() string {
	if w == nil {
		return ""
	}
	return w.publicB64
}

// WebPushMessage is the closed JSON body a service worker showNotification
// consumes. Data is the same envelope native push uses.
type WebPushMessage struct {
	Endpoint string
	P256dh   string
	Auth     string
	Title    string
	Body     string
	Urgency  string
	Data     EnvelopeData
}

type webPushPayload struct {
	Title string       `json:"title"`
	Body  string       `json:"body"`
	Data  EnvelopeData `json:"data"`
}

// Send encrypts and POSTs one notification. Ticket ID is empty: Web Push has
// no receipt poll. 404/410 map to InvalidTokenError so the worker prunes.
func (w *WebPush) Send(ctx context.Context, msg WebPushMessage) error {
	if w == nil {
		return errors.New("push webpush transport unavailable")
	}
	// allowLoopbackHTTP remains true for Send: unit tests inject an http.Client
	// against httptest (HTTP loopback), and the production default client still
	// refuses those dials via SafeDialContext (codex round-16 #6). Registration
	// is HTTPS-only and never persists a loopback endpoint.
	endpoint, err := validatePushEndpoint(msg.Endpoint, true)
	if err != nil {
		return &PayloadError{Field: "endpoint", Reason: "invalid"}
	}
	uaPub, err := decodeKeyMaterial(msg.P256dh)
	if err != nil || len(uaPub) != 65 || uaPub[0] != 0x04 {
		return &PayloadError{Field: "p256dh", Reason: "invalid"}
	}
	authSecret, err := decodeKeyMaterial(msg.Auth)
	if err != nil || len(authSecret) < 16 || len(authSecret) > 32 {
		return &PayloadError{Field: "auth", Reason: "invalid"}
	}
	plain, err := encodeWebPushJSON(msg)
	if err != nil {
		return err
	}
	body, err := encryptAES128GCM(uaPub, authSecret, plain)
	if err != nil {
		return &TransientError{Operation: "send", Detail: "encrypt"}
	}
	token, err := w.vapidJWT(endpoint)
	if err != nil {
		return &TransientError{Operation: "send", Detail: "vapid"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return &PayloadError{Field: "endpoint", Reason: "invalid"}
	}
	req.Header.Set("TTL", strconv.Itoa(webPushTTLSeconds))
	req.Header.Set("Urgency", webPushUrgency(msg.Urgency))
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Authorization", "vapid t="+token+", k="+w.publicB64)

	resp, err := w.http.Do(req)
	if err != nil {
		return &TransientError{Operation: "send", Detail: "network"}
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.CopyN(io.Discard, resp.Body, 4<<10)
	return classifyWebPushStatus(resp.StatusCode, resp.Header.Get("Retry-After"))
}

func encodeWebPushJSON(msg WebPushMessage) ([]byte, error) {
	if err := validateEnvelope(msg.Title, msg.Body, msg.Data); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(webPushPayload{Title: msg.Title, Body: msg.Body, Data: msg.Data})
	if err != nil {
		return nil, &PayloadError{Field: "body", Reason: "encode"}
	}
	if len(raw) > webPushMaxBody {
		return nil, &PayloadError{Field: "body", Reason: "too_large"}
	}
	return raw, nil
}

func validateEnvelope(title, body string, data EnvelopeData) error {
	if title == "" || len(title) > MaxTitleBytes {
		return &PayloadError{Field: "title", Reason: "length"}
	}
	if body == "" || len(body) > MaxBodyBytes {
		return &PayloadError{Field: "body", Reason: "length"}
	}
	if data.Schema == "" || data.NotificationID == "" || data.Event == "" || data.Route == "" {
		return &PayloadError{Field: "data", Reason: "required"}
	}
	if !validSameOriginRoute(data.Route) {
		return &PayloadError{Field: "route", Reason: "invalid"}
	}
	return nil
}

func validSameOriginRoute(route string) bool {
	if len(route) > MaxRouteBytes || !strings.HasPrefix(route, "/") || strings.HasPrefix(route, "//") || strings.HasPrefix(route, `/\`) {
		return false
	}
	parsed, err := url.ParseRequestURI(route)
	return err == nil && !parsed.IsAbs() && parsed.Host == "" && !strings.Contains(route, `\`)
}

func classifyWebPushStatus(code int, retryAfter string) error {
	switch {
	case code == http.StatusCreated || code == http.StatusAccepted || code == http.StatusOK:
		return nil
	case code == http.StatusNotFound || code == http.StatusGone:
		return &InvalidTokenError{Code: strconv.Itoa(code)}
	case code == http.StatusTooManyRequests:
		return &RateLimitedError{Operation: "send", RetryAfter: parseRetryAfter(retryAfter)}
	case code == http.StatusRequestTimeout || code >= 500:
		return &TransientError{Operation: "send", Detail: "http_" + strconv.Itoa(code)}
	case code == http.StatusBadRequest || code == http.StatusRequestEntityTooLarge:
		return &PayloadError{Field: "body", Reason: "rejected"}
	default:
		return &PermanentError{Operation: "send", Code: "http_" + strconv.Itoa(code)}
	}
}

func webPushUrgency(v string) string {
	switch v {
	case "critical":
		return "high"
	case "important":
		return "normal"
	default:
		return "low"
	}
}

func (w *WebPush) vapidJWT(endpoint string) (string, error) {
	origin, err := endpointOrigin(endpoint)
	if err != nil {
		return "", err
	}
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"aud": origin,
		"exp": now.Add(vapidMaxAge).Unix(),
		"sub": w.subscriber,
	})
	return token.SignedString(w.privateKey)
}

func endpointOrigin(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", errors.New("invalid endpoint")
	}
	return u.Scheme + "://" + u.Host, nil
}

func validateVAPIDSubscriber(sub string) error {
	u, err := url.Parse(sub)
	if err != nil || u.Opaque == "" && u.Host == "" {
		if strings.HasPrefix(sub, "mailto:") && len(sub) > len("mailto:") && len(sub) <= 256 {
			return nil
		}
		return errors.New("push: BEX_WEBPUSH_SUBSCRIBER must be a mailto: or https: URI")
	}
	if (u.Scheme != "mailto" && u.Scheme != "https") || len(sub) > 256 {
		return errors.New("push: BEX_WEBPUSH_SUBSCRIBER must be a mailto: or https: URI")
	}
	return nil
}

func parseVAPIDPrivate(s string) (*ecdsa.PrivateKey, error) {
	raw, err := decodeKeyMaterial(s)
	if err != nil {
		return nil, errors.New("push: BEX_WEBPUSH_VAPID_PRIVATE_KEY is malformed")
	}
	if len(raw) == 33 && raw[0] == 0 {
		raw = raw[1:]
	}
	if len(raw) != 32 {
		return nil, errors.New("push: BEX_WEBPUSH_VAPID_PRIVATE_KEY must be a 32-byte P-256 scalar")
	}
	curve := elliptic.P256()
	d := new(big.Int).SetBytes(raw)
	x, y := curve.ScalarBaseMult(raw)
	if x == nil {
		return nil, errors.New("push: BEX_WEBPUSH_VAPID_PRIVATE_KEY is not a valid P-256 scalar")
	}
	return &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: curve, X: x, Y: y}, D: d}, nil
}

func decodeKeyMaterial(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil && len(b) > 0 {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil && len(b) > 0 {
		return b, nil
	}
	return nil, errors.New("invalid base64url")
}

func ValidatePublicPushEndpoint(raw string) (string, error) {
	return validatePushEndpoint(raw, false)
}

func DecodeSubscriptionKey(s string) ([]byte, error) {
	return decodeKeyMaterial(s)
}

func validatePushEndpoint(raw string, allowLoopbackHTTP bool) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.User != nil || u.Host == "" || len(raw) > 2048 {
		return "", errors.New("invalid")
	}
	switch u.Scheme {
	case "https":
		return u.String(), nil
	case "http":
		if allowLoopbackHTTP && isLoopbackHost(u.Hostname()) {
			return u.String(), nil
		}
	}
	return "", errors.New("invalid")
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// encryptAES128GCM implements RFC 8291 (RFC 8188 aes128gcm, one record).
func encryptAES128GCM(uaPublic, authSecret, plaintext []byte) ([]byte, error) {
	asKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	uaECDH, err := ecdh.P256().NewPublicKey(uaPublic)
	if err != nil {
		return nil, err
	}
	secret, err := asKey.ECDH(uaECDH)
	if err != nil {
		return nil, err
	}
	asPublic := asKey.PublicKey().Bytes()
	authInfo := append([]byte("WebPush: info\x00"), append(uaPublic, asPublic...)...)
	ikm, err := hkdfExpand(hkdf.Extract(sha256.New, secret, authSecret), authInfo, 32)
	if err != nil {
		return nil, err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	prk := hkdf.Extract(sha256.New, ikm, salt)
	cek, err := hkdfExpand(prk, []byte("Content-Encoding: aes128gcm\x00"), 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdfExpand(prk, []byte("Content-Encoding: nonce\x00"), 12)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	record := append(append([]byte{}, plaintext...), 0x02)
	ciphertext := gcm.Seal(nil, nonce, record, nil)
	header := make([]byte, 16+4+1+len(asPublic))
	copy(header, salt)
	binary.BigEndian.PutUint32(header[16:20], webPushRecordSize)
	header[20] = byte(len(asPublic))
	copy(header[21:], asPublic)
	out := append(header, ciphertext...)
	if len(out) > webPushRecordSize+21+len(asPublic) {
		return nil, errors.New("record too large")
	}
	return out, nil
}

func hkdfExpand(prk, info []byte, n int) ([]byte, error) {
	out := make([]byte, n)
	if _, err := io.ReadFull(hkdf.Expand(sha256.New, prk, info), out); err != nil {
		return nil, err
	}
	return out, nil
}
