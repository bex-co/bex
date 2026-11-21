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

package objectstorage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// A bucket's lifecycle rules are stored in object_buckets.lifecycle in the shape
// Cloudflare's R2 uses (ADR112 D3, D5; the S3 gateway converts to and from it), and
// the hourly sweep executes the ones that delete objects and abort incomplete
// multipart uploads. storageClassTransitions are refused at the route (bex has one
// storage class).

// Lifecycle condition types of deleteObjectsTransition.
const (
	LifecycleAge  = "Age"
	LifecycleDate = "Date"
)

// LifecycleRule is one stored lifecycle rule. Unknown fields are tolerated and
// not kept.
type LifecycleRule struct {
	ID         string              `json:"id"`
	Enabled    bool                `json:"enabled"`
	Conditions LifecycleConditions `json:"conditions"`
	// DeleteObjectsTransition is nil on a rule that only transitions storage
	// class or aborts multipart uploads, which do nothing here.
	DeleteObjectsTransition *LifecycleDelete `json:"deleteObjectsTransition,omitempty"`
	// StorageClassTransitions and AbortMultipartUploadsTransition are kept
	// verbatim so a stored rule reads back as it was written.
	StorageClassTransitions         json.RawMessage `json:"storageClassTransitions,omitempty"`
	AbortMultipartUploadsTransition json.RawMessage `json:"abortMultipartUploadsTransition,omitempty"`
}

// LifecycleConditions says which objects a rule applies to.
type LifecycleConditions struct {
	// Prefix limits the rule to keys that start with it, relative to the bucket
	// (not the Wasabi key). Empty matches every object.
	Prefix string `json:"prefix"`
}

// LifecycleDelete is the deleteObjectsTransition of a rule.
type LifecycleDelete struct {
	Condition LifecycleDeleteCondition `json:"condition"`
}

// LifecycleDeleteCondition is when a rule deletes an object: Type is Age, and
// MaxAge is the seconds since the object was last modified; or Type is Date,
// and Date is an RFC 3339 date and time (or a bare YYYY-MM-DD, midnight UTC),
// from which every matching object is deleted whatever its age, as S3's and
// R2's date expiration does. MaxAge is a pointer so that a rule with no maxAge,
// which must delete nothing, is not read as maxAge 0, which deletes every
// matching object at the next sweep.
type LifecycleDeleteCondition struct {
	Type   string `json:"type"`
	MaxAge *int64 `json:"maxAge,omitempty"`
	Date   string `json:"date,omitempty"`
}

// MaxLifecycleAgeSeconds is the longest maxAge a rule may carry: 100 years. A
// larger one is a "never expire" idiom that must not be able to overflow.
const MaxLifecycleAgeSeconds = 100 * 365 * 24 * 3600

var (
	// ErrInvalidLifecycle refuses a lifecycle document or rule bex cannot store
	// or run.
	ErrInvalidLifecycle = errors.New("invalid lifecycle rule")
	// ErrLifecycleOverCap refuses a rule set over MaxLifecycleRules.
	ErrLifecycleOverCap = errors.New("the lifecycle configuration is over bex's limit")
	// ErrLifecycleStorageClass refuses a rule that moves objects to another
	// storage class, which bex does not have.
	ErrLifecycleStorageClass = errors.New("storage class transitions are not offered")
)

const maxLifecycleIDBytes = 255

// ValidateLifecycle checks a rule set before it is stored: at most
// MaxLifecycleRules, each with a unique id of 1 to 255 bytes, a prefix without
// control characters, and an expiry or an abort of incomplete multipart uploads. A storage class
// transition is ErrLifecycleStorageClass and a set over the cap is
// ErrLifecycleOverCap; anything else is an ErrInvalidLifecycle naming the rule.
// An expiry is judged by Validate, the check the sweep makes before it runs a
// rule, so a rule that is stored is a rule that runs.
//
// The S3 gateway is the only caller and hands the message to the client as it
// is, so it names the elements of the S3 document the client sent (ID, Prefix,
// Expiration, AbortIncompleteMultipartUpload), never the stored fields.
func ValidateLifecycle(rules []LifecycleRule) error {
	if len(rules) > MaxLifecycleRules {
		return fmt.Errorf("%w: %d rules, at most %d", ErrLifecycleOverCap, len(rules), MaxLifecycleRules)
	}
	seen := map[string]bool{}
	for i, rule := range rules {
		bad := func(format string, args ...any) error {
			return fmt.Errorf("%w: lifecycle rule %d: %s", ErrInvalidLifecycle, i+1, fmt.Sprintf(format, args...))
		}
		if rule.ID == "" || len(rule.ID) > maxLifecycleIDBytes {
			return bad("ID must be 1 to %d bytes", maxLifecycleIDBytes)
		}
		if seen[rule.ID] {
			return bad("ID %q is used by more than one rule", rule.ID)
		}
		seen[rule.ID] = true
		if len(rule.Conditions.Prefix) > MaxKeyBytes || strings.ContainsFunc(rule.Conditions.Prefix, unicode.IsControl) {
			return bad("Prefix must be at most %d bytes with no control character", MaxKeyBytes)
		}
		if hasTransitions(rule.StorageClassTransitions) {
			return fmt.Errorf("%w: lifecycle rule %q moves objects to another storage class, and bex has one", ErrLifecycleStorageClass, rule.ID)
		}
		abort, err := abortTransition(rule)
		if err == nil && abort != nil {
			err = (LifecycleRule{ID: rule.ID, DeleteObjectsTransition: abort}).Validate()
		}
		if err != nil {
			return bad("AbortIncompleteMultipartUpload needs a valid DaysAfterInitiation")
		}
		if rule.DeleteObjectsTransition == nil && abort == nil {
			return bad("a rule needs an Expiration or an AbortIncompleteMultipartUpload")
		}
		if rule.Validate() != nil {
			return bad("Expiration needs a valid Days or Date")
		}
	}
	return nil
}

// hasTransitions reports whether a rule's storageClassTransitions holds any.
func hasTransitions(raw json.RawMessage) bool {
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		return len(raw) > 0 // not a list: refuse it rather than store it unread
	}
	return len(list) > 0
}

// abortTransition reads a rule's abortMultipartUploadsTransition, which has the
// same shape as an expiry, or nil when the rule has none.
func abortTransition(rule LifecycleRule) (*LifecycleDelete, error) {
	if len(rule.AbortMultipartUploadsTransition) == 0 || string(rule.AbortMultipartUploadsTransition) == "null" {
		return nil, nil
	}
	var t LifecycleDelete
	if err := json.Unmarshal(rule.AbortMultipartUploadsTransition, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// abortRule is an enabled AbortIncompleteMultipartUpload rule of a bucket: the uploads under
// prefix that were started more than maxAge ago are aborted.
type abortRule struct {
	prefix string
	maxAge time.Duration
}

// abortRules reads a bucket's stored rules for the sweep: the enabled ones with an age
// condition. A rule that does not parse, or whose abort carries no age, is skipped and
// logged, as for the deleting rules.
func abortRules(bucketID string, raw json.RawMessage, logf func(string, ...any)) []abortRule {
	parsed, _, err := parseRules(raw)
	if err != nil {
		logf("object storage sweep: bucket %s: skipping its lifecycle rules: %v", bucketID, err)
		return nil
	}
	var out []abortRule
	for _, r := range parsed {
		if !r.Enabled {
			continue
		}
		t, err := abortTransition(r)
		if err != nil || t == nil || t.Condition.Type != LifecycleAge || t.Condition.MaxAge == nil ||
			*t.Condition.MaxAge < 0 || *t.Condition.MaxAge > MaxLifecycleAgeSeconds {
			if t != nil || err != nil {
				logf("object storage sweep: bucket %s: skipping the abort rule %q: it needs an age in seconds", bucketID, r.ID)
			}
			continue
		}
		out = append(out, abortRule{prefix: r.Conditions.Prefix, maxAge: time.Duration(*t.Condition.MaxAge) * time.Second})
	}
	return out
}

// ParseLifecycle reads a bucket's stored lifecycle JSON, an array of rules.
// Empty input, null and [] are no rules. It checks the JSON shape only, so
// every error wraps ErrInvalidLifecycle; Validate judges whether a rule is
// one bex can run. Unknown fields are tolerated.
func ParseLifecycle(raw json.RawMessage) ([]LifecycleRule, error) {
	rules, bad, err := parseRules(raw)
	if err != nil {
		return nil, err
	}
	if len(bad) > 0 {
		return nil, bad[0]
	}
	return rules, nil
}

// parseRules reads each rule of the document on its own, so one rule of the
// wrong shape is reported in bad and does not take its neighbours with it. err
// is a document that is not an array of objects.
func parseRules(raw json.RawMessage) (rules []LifecycleRule, bad []error, err error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil, nil
	}
	var parts []json.RawMessage
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidLifecycle, err)
	}
	for i, part := range parts {
		var r LifecycleRule
		if err := json.Unmarshal(part, &r); err != nil {
			bad = append(bad, fmt.Errorf("%w: rule %d: %v", ErrInvalidLifecycle, i, err))
			continue
		}
		rules = append(rules, r)
	}
	return rules, bad, nil
}

// Validate reports why bex cannot run a rule's deleteObjectsTransition: a type
// other than Age or Date, an Age with no maxAge or a negative one, or a Date that
// is not a date. A rule without a deleteObjectsTransition is valid and does
// nothing.
func (r LifecycleRule) Validate() error {
	if r.DeleteObjectsTransition == nil {
		return nil
	}
	c := r.DeleteObjectsTransition.Condition
	switch c.Type {
	case LifecycleAge:
		if c.MaxAge == nil || *c.MaxAge < 0 || *c.MaxAge > MaxLifecycleAgeSeconds {
			return fmt.Errorf("%w: rule %q: an Age condition needs a maxAge in seconds from 0 to %d", ErrInvalidLifecycle, r.ID, MaxLifecycleAgeSeconds)
		}
	case LifecycleDate:
		if _, err := parseLifecycleDate(c.Date); err != nil {
			return fmt.Errorf("%w: rule %q: a Date condition needs an RFC 3339 date: %v", ErrInvalidLifecycle, r.ID, err)
		}
	default:
		return fmt.Errorf("%w: rule %q: the condition type %q is not Age or Date", ErrInvalidLifecycle, r.ID, c.Type)
	}
	return nil
}

func parseLifecycleDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse(time.DateOnly, s)
}

// expires reports whether the rule, already known to be enabled and valid,
// deletes an object with this bucket-relative key and last-modified time at
// now. Age deletes an object that was last modified more than maxAge seconds
// ago; Date deletes every matching object once the date has passed.
func (r LifecycleRule) expires(key string, lastModified, now time.Time) bool {
	if !strings.HasPrefix(key, r.Conditions.Prefix) {
		return false
	}
	c := r.DeleteObjectsTransition.Condition
	if c.Type == LifecycleAge {
		// Whole seconds against maxAge: a Duration of a large maxAge overflows
		// and would delete everything.
		return int64(now.Sub(lastModified)/time.Second) > *c.MaxAge
	}
	date, _ := parseLifecycleDate(c.Date) // Validate parsed it
	return !now.Before(date)
}

// sweepLifecycle is a bucket's rules as one sweep runs them: the enabled rules
// that delete objects and validate, in stored order, each with the deletions it
// has made so far this sweep.
type sweepLifecycle struct {
	rules []LifecycleRule
	used  []int
}

// newSweepLifecycle reads a bucket's stored rules for the sweep. A document or a
// rule that does not parse or validate is skipped and logged, never an error:
// the routes validate what they store, so a bad one means the row was edited or
// a later version wrote a shape this one does not know, and it must not stop
// the sweep or the bucket's other rules. It returns nil when no rule would
// delete anything.
func newSweepLifecycle(bucketID string, raw json.RawMessage, logf func(string, ...any)) *sweepLifecycle {
	parsed, bad, err := parseRules(raw)
	if err != nil {
		logf("object storage sweep: bucket %s: skipping its lifecycle rules: %v", bucketID, err)
		return nil
	}
	for _, err := range bad {
		logf("object storage sweep: bucket %s: skipping a lifecycle rule: %v", bucketID, err)
	}
	l := &sweepLifecycle{}
	for _, r := range parsed {
		if !r.Enabled || r.DeleteObjectsTransition == nil {
			continue // disabled, or a transition that does nothing in bex
		}
		if err := r.Validate(); err != nil {
			logf("object storage sweep: bucket %s: skipping a lifecycle rule: %v", bucketID, err)
			continue
		}
		l.rules = append(l.rules, r)
	}
	if len(l.rules) == 0 {
		return nil
	}
	l.used = make([]int, len(l.rules))
	return l
}

// claim reports whether some rule deletes the object, and counts the deletion
// against the first such rule that still has room under cap. An object that
// several rules match is deleted once, charged to one rule.
func (l *sweepLifecycle) claim(key string, lastModified, now time.Time, cap int) bool {
	if l == nil {
		return false
	}
	for i, r := range l.rules {
		if l.used[i] < cap && r.expires(key, lastModified, now) {
			l.used[i]++
			return true
		}
	}
	return false
}
