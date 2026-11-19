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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The bucket operations and the bucket configuration (ADR112 D1, D3, D4, D5): a
// thin adapter of objectstorage.Service, as ADR006 asks.

func init() {
	handlers["ListBuckets"] = (*request).listBuckets
	handlers["CreateBucket"] = (*request).createBucket
	handlers["HeadBucket"] = (*request).headBucket
	handlers["DeleteBucket"] = (*request).deleteBucket
	handlers["GetBucketLocation"] = (*request).getBucketLocation
	handlers["GetBucketCors"] = (*request).getBucketCors
	handlers["PutBucketCors"] = (*request).putBucketCors
	handlers["DeleteBucketCors"] = (*request).deleteBucketCors
	handlers["GetBucketLifecycleConfiguration"] = (*request).getBucketLifecycle
	handlers["PutBucketLifecycleConfiguration"] = (*request).putBucketLifecycle
	handlers["DeleteBucketLifecycle"] = (*request).deleteBucketLifecycle
}

// maxConfigBody bounds a bucket configuration document: 1,000 lifecycle rules or 100
// CORS rules of 64 KiB, as XML.
const maxConfigBody = 1 << 20

const s3Time = "2006-01-02T15:04:05.000Z"

// serviceError maps what the object store says to S3's answers (ADR112 D3). Anything
// it does not name is an InternalError, logged with the request id and never shown.
func serviceError(w http.ResponseWriter, err error) *Error {
	var notOffered *objectstorage.RegionNotOfferedError
	var quota *objectstorage.QuotaError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, objectstorage.ErrBucketNotFound):
		return newError(http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist.")
	case errors.Is(err, objectstorage.ErrBucketExists):
		return newError(http.StatusConflict, "BucketAlreadyOwnedByYou", "Your previous request to create the named bucket succeeded and you already own it.")
	case errors.Is(err, objectstorage.ErrBucketNotEmpty):
		return newError(http.StatusConflict, "BucketNotEmpty", "The bucket you tried to delete is not empty.")
	case errors.Is(err, objectstorage.ErrTooManyBuckets):
		return newError(http.StatusBadRequest, "TooManyBuckets", fmt.Sprintf("You have attempted to create more buckets than allowed (%d).", objectstorage.MaxUserBuckets))
	case errors.Is(err, objectstorage.ErrSystemBucket):
		return errAccessDenied("render-<region> is the namespace of the Render CLI's object storage and cannot be deleted.")
	case errors.Is(err, objectstorage.ErrBucketDeleting):
		return newError(http.StatusConflict, "OperationAborted", "A conflicting conditional operation is currently in progress against this resource. Try again.")
	case errors.Is(err, objectstorage.ErrInvalidBucketName), errors.Is(err, objectstorage.ErrReservedBucketName):
		return newError(http.StatusBadRequest, "InvalidBucketName", "The specified bucket is not valid: a name is 3 to 63 lower-case letters, digits and hyphens, and render-* and cloudflare are reserved.")
	case errors.As(err, &notOffered):
		return newError(http.StatusBadRequest, "InvalidLocationConstraint", notOffered.Error())
	case errors.Is(err, objectstorage.ErrInvalidKey):
		return errInvalidArgument("The object key is not valid.")
	case errors.Is(err, objectstorage.ErrObjectNotFound):
		return newError(http.StatusNotFound, "NoSuchKey", "The specified key does not exist.")
	case errors.As(err, &quota):
		return newError(http.StatusForbidden, "QuotaExceeded", quota.Error())
	case errors.Is(err, objectstorage.ErrObjectTooLarge):
		return newError(http.StatusBadRequest, "EntityTooLarge", "Your proposed upload exceeds the maximum allowed object size.")
	case errors.Is(err, objectstorage.ErrCORSOverCap), errors.Is(err, objectstorage.ErrInvalidCORS),
		errors.Is(err, objectstorage.ErrLifecycleOverCap), errors.Is(err, objectstorage.ErrInvalidLifecycle):
		return errInvalidRequest(err.Error())
	case errors.Is(err, objectstorage.ErrLifecycleStorageClass):
		return errNotImplemented("Storage class transitions are not implemented; bex has one storage class")
	case errors.Is(err, objectstorage.ErrNotConfigured):
		return errServiceUnavailable("object storage is not configured on this deployment")
	}
	log.Printf("s3gw: request %s: %v", w.Header().Get("x-amz-request-id"), err)
	return errInternal()
}

func (rq *request) fail(err error) *Error { return serviceError(rq.w, err) }

// bucketRow is the key's workspace's bucket the path names. A name another workspace
// holds is simply no bucket here, so nothing about other workspaces can be probed.
func (rq *request) bucketRow() (store.ObjectBucket, *Error) {
	b, err := rq.g.cfg.Service.ResolveBucket(rq.ctx, rq.workspaceID, rq.bucket)
	if err != nil {
		return store.ObjectBucket{}, rq.fail(err)
	}
	return b, nil
}

func (rq *request) listBuckets() *Error {
	rows, err := rq.g.cfg.Service.ListBuckets(rq.ctx, rq.workspaceID)
	if err != nil {
		return rq.fail(err)
	}
	doc := listAllMyBucketsXML{Xmlns: s3Namespace, Owner: ownerXML{ID: rq.workspaceID, DisplayName: rq.workspaceID}}
	for _, b := range rows {
		doc.Buckets.Bucket = append(doc.Buckets.Bucket, bucketXML{Name: b.Name, CreationDate: b.CreatedAt.UTC().Format(s3Time)})
	}
	return writeXML(rq.w, http.StatusOK, doc)
}

func (rq *request) createBucket() *Error {
	body, e := rq.readSmall(maxConfigBody)
	if e != nil {
		return e
	}
	var cfg createBucketConfigurationXML
	if len(strings.TrimSpace(string(body))) > 0 {
		if e := decodeXML(body, &cfg); e != nil {
			return e
		}
	}
	region, err := objectstorage.RegionFromLocationConstraint(cfg.LocationConstraint)
	if err != nil {
		return rq.fail(err)
	}
	b, err := rq.g.cfg.Service.CreateBucket(rq.ctx, rq.workspaceID, rq.bucket, region, "")
	if err != nil {
		return rq.fail(err)
	}
	// The allowed row is written once the bucket exists, so a create that failed
	// (a name taken, an invalid one, the workspace at its limit) records none.
	rq.g.cfg.Base.RecordAllowedWrite(rq.ctx, objectstorage.AuditVerbCreateBucket, core.RelCanCreate, rq.workspaceID, core.BucketTarget(b.Name), "")
	rq.w.Header().Set("Location", "/"+b.Name)
	rq.w.WriteHeader(http.StatusOK)
	return nil
}

func (rq *request) headBucket() *Error {
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	rq.w.Header().Set("x-amz-bucket-region", b.Region)
	rq.w.WriteHeader(http.StatusOK)
	return nil
}

func (rq *request) deleteBucket() *Error {
	if err := rq.g.cfg.Service.DeleteBucket(rq.ctx, rq.workspaceID, rq.bucket); err != nil {
		return rq.fail(err)
	}
	rq.w.WriteHeader(http.StatusNoContent)
	return nil
}

func (rq *request) getBucketLocation() *Error {
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	return writeXML(rq.w, http.StatusOK, locationConstraintXML{Xmlns: s3Namespace, Value: b.Region})
}

func (rq *request) getBucketCors() *Error {
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	rules, err := objectstorage.ParseCORS(b.CORS)
	if err != nil {
		return rq.fail(err)
	}
	if len(rules) == 0 {
		return newError(http.StatusNotFound, "NoSuchCORSConfiguration", "The CORS configuration does not exist")
	}
	return writeXML(rq.w, http.StatusOK, corsToXML(rules))
}

func (rq *request) putBucketCors() *Error {
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	body, e := rq.readSmall(maxConfigBody)
	if e != nil {
		return e
	}
	var doc corsConfigurationXML
	if e := decodeXML(body, &doc); e != nil {
		return e
	}
	if len(doc.Rules) == 0 {
		return errMalformedXML("A CORS configuration needs at least one CORSRule.")
	}
	if err := rq.g.cfg.Service.SetCORS(rq.ctx, rq.workspaceID, b, corsFromXML(doc)); err != nil {
		return rq.fail(err)
	}
	rq.w.WriteHeader(http.StatusOK)
	return nil
}

func (rq *request) deleteBucketCors() *Error {
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	if err := rq.g.cfg.Service.SetCORS(rq.ctx, rq.workspaceID, b, nil); err != nil {
		return rq.fail(err)
	}
	rq.w.WriteHeader(http.StatusNoContent)
	return nil
}

// corsFromXML maps S3's rules onto the stored shape one to one, keeping each rule's
// ID so a Get returns what a Put sent.
func corsFromXML(doc corsConfigurationXML) []objectstorage.CORSRule {
	rules := make([]objectstorage.CORSRule, 0, len(doc.Rules))
	for _, r := range doc.Rules {
		rules = append(rules, objectstorage.CORSRule{
			ID:            r.ID,
			Allowed:       objectstorage.CORSAllowed{Origins: r.AllowedOrigins, Methods: r.AllowedMethods, Headers: r.AllowedHeaders},
			ExposeHeaders: r.ExposeHeaders, MaxAgeSeconds: r.MaxAgeSeconds,
		})
	}
	return rules
}

func corsToXML(rules []objectstorage.CORSRule) corsConfigurationXML {
	doc := corsConfigurationXML{Xmlns: s3Namespace}
	for _, r := range rules {
		doc.Rules = append(doc.Rules, corsRuleXML{
			ID: r.ID, AllowedOrigins: r.Allowed.Origins, AllowedMethods: r.Allowed.Methods, AllowedHeaders: r.Allowed.Headers,
			ExposeHeaders: r.ExposeHeaders, MaxAgeSeconds: r.MaxAgeSeconds,
		})
	}
	return doc
}

func (rq *request) getBucketLifecycle() *Error {
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	rules, err := objectstorage.ParseLifecycle(b.Lifecycle)
	if err != nil {
		return rq.fail(err)
	}
	if len(rules) == 0 {
		return newError(http.StatusNotFound, "NoSuchLifecycleConfiguration", "The lifecycle configuration does not exist")
	}
	return writeXML(rq.w, http.StatusOK, lifecycleToXML(rules))
}

func (rq *request) putBucketLifecycle() *Error {
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	body, e := rq.readSmall(maxConfigBody)
	if e != nil {
		return e
	}
	var doc lifecycleConfigurationXML
	if e := decodeXML(body, &doc); e != nil {
		return e
	}
	if len(doc.Rules) == 0 {
		return errMalformedXML("A lifecycle configuration needs at least one Rule.")
	}
	rules, e := lifecycleFromXML(doc)
	if e != nil {
		return e
	}
	if err := rq.g.cfg.Service.SetLifecycle(rq.ctx, rq.workspaceID, b, rules); err != nil {
		return rq.fail(err)
	}
	rq.w.WriteHeader(http.StatusOK)
	return nil
}

func (rq *request) deleteBucketLifecycle() *Error {
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	if err := rq.g.cfg.Service.SetLifecycle(rq.ctx, rq.workspaceID, b, nil); err != nil {
		return rq.fail(err)
	}
	rq.w.WriteHeader(http.StatusNoContent)
	return nil
}

const secondsPerDay = 24 * 3600

// lifecycleFromXML maps S3's rules onto the stored shape the sweep runs (ADR112 D3):
// Days is a maxAge in seconds, a Date stays a Date, Status is enabled, Filter.Prefix
// (or the legacy Prefix) is the condition's prefix, and AbortIncompleteMultipartUpload
// is the abort transition. A Transition, a NoncurrentVersion action, a tag or size filter
// and an ExpiredObjectDeleteMarker answer 501: bex has one storage class, no versions
// and no tags.
func lifecycleFromXML(doc lifecycleConfigurationXML) ([]objectstorage.LifecycleRule, *Error) {
	rules := make([]objectstorage.LifecycleRule, 0, len(doc.Rules))
	for i, r := range doc.Rules {
		if len(r.Transition) > 0 || len(r.NoncurrentVersionTransition) > 0 {
			return nil, errNotImplemented("Storage class transitions are not implemented; bex has one storage class")
		}
		if r.NoncurrentVersionExpiration != nil {
			return nil, errNotImplemented("NoncurrentVersionExpiration is not implemented; bex has no versioning")
		}
		if r.Filter != nil && (r.Filter.Tag != nil || r.Filter.And != nil || r.Filter.ObjectSizeGreaterThan != nil || r.Filter.ObjectSizeLessThan != nil) {
			return nil, errNotImplemented("Lifecycle filters by tag or size are not implemented; filter by Prefix")
		}
		rule := objectstorage.LifecycleRule{ID: r.ID}
		if rule.ID == "" {
			rule.ID = fmt.Sprintf("rule-%d", i+1)
		}
		switch r.Status {
		case "Enabled":
			rule.Enabled = true
		case "Disabled":
		default:
			return nil, errMalformedXML("A lifecycle rule's Status must be Enabled or Disabled.")
		}
		switch {
		case r.Filter != nil && r.Filter.Prefix != nil && r.Prefix != nil:
			return nil, errMalformedXML("A lifecycle rule takes Filter or a Prefix, not both.")
		case r.Filter != nil && r.Filter.Prefix != nil:
			rule.Conditions.Prefix = *r.Filter.Prefix
		case r.Prefix != nil:
			rule.Conditions.Prefix = *r.Prefix
		}
		if x := r.Expiration; x != nil {
			if x.ExpiredObjectDeleteMarker != nil && x.Days == nil && x.Date == "" {
				return nil, errNotImplemented("ExpiredObjectDeleteMarker is not implemented; bex has no versioning")
			}
			cond, e := expirationCondition(x)
			if e != nil {
				return nil, e
			}
			rule.DeleteObjectsTransition = &objectstorage.LifecycleDelete{Condition: cond}
		}
		if a := r.AbortIncompleteMultipartUpload; a != nil {
			if a.DaysAfterInitiation == nil || *a.DaysAfterInitiation < 1 || *a.DaysAfterInitiation > maxLifecycleDays {
				return nil, errInvalidArgument(fmt.Sprintf("DaysAfterInitiation must be between 1 and %d.", maxLifecycleDays))
			}
			age := *a.DaysAfterInitiation * secondsPerDay
			raw, err := json.Marshal(objectstorage.LifecycleDelete{Condition: objectstorage.LifecycleDeleteCondition{Type: objectstorage.LifecycleAge, MaxAge: &age}})
			if err != nil {
				return nil, errInternal()
			}
			rule.AbortMultipartUploadsTransition = raw
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// maxLifecycleDays is the largest Days (100 years, MaxLifecycleAgeSeconds): a larger
// "never expire" idiom would overflow a duration and delete every object.
const maxLifecycleDays = objectstorage.MaxLifecycleAgeSeconds / secondsPerDay

func expirationCondition(x *expirationXML) (objectstorage.LifecycleDeleteCondition, *Error) {
	switch {
	case x.Days != nil && x.Date != "":
		return objectstorage.LifecycleDeleteCondition{}, errMalformedXML("An Expiration takes Days or Date, not both.")
	case x.Days != nil:
		if *x.Days < 1 || *x.Days > maxLifecycleDays {
			return objectstorage.LifecycleDeleteCondition{}, errInvalidArgument(fmt.Sprintf("Expiration Days must be between 1 and %d.", maxLifecycleDays))
		}
		age := *x.Days * secondsPerDay
		return objectstorage.LifecycleDeleteCondition{Type: objectstorage.LifecycleAge, MaxAge: &age}, nil
	case x.Date != "":
		t, err := time.Parse(time.RFC3339, x.Date)
		if err != nil {
			return objectstorage.LifecycleDeleteCondition{}, errInvalidArgument("Expiration Date must be an ISO 8601 date.")
		}
		if t.UTC().Hour() != 0 || t.UTC().Minute() != 0 || t.UTC().Second() != 0 || t.UTC().Nanosecond() != 0 {
			return objectstorage.LifecycleDeleteCondition{}, errInvalidArgument("'Date' must be at midnight GMT.")
		}
		return objectstorage.LifecycleDeleteCondition{Type: objectstorage.LifecycleDate, Date: x.Date}, nil
	}
	return objectstorage.LifecycleDeleteCondition{}, errMalformedXML("An Expiration takes Days or Date.")
}

// lifecycleToXML is the reverse: what a Put sent reads back, and a rule written through
// Contract B in seconds reads as the whole days that cover it.
func lifecycleToXML(rules []objectstorage.LifecycleRule) lifecycleConfigurationXML {
	doc := lifecycleConfigurationXML{Xmlns: s3Namespace}
	for _, r := range rules {
		out := lifecycleRuleXML{ID: r.ID, Status: "Disabled", Filter: &filterXML{Prefix: strPtr(r.Conditions.Prefix)}}
		if r.Enabled {
			out.Status = "Enabled"
		}
		if t := r.DeleteObjectsTransition; t != nil {
			x := &expirationXML{}
			switch t.Condition.Type {
			case objectstorage.LifecycleAge:
				if t.Condition.MaxAge != nil {
					days := (*t.Condition.MaxAge + secondsPerDay - 1) / secondsPerDay
					x.Days = &days
				}
			case objectstorage.LifecycleDate:
				x.Date = formatLifecycleDate(t.Condition.Date)
			}
			out.Expiration = x
		}
		if abort := abortDays(r); abort != nil {
			out.AbortIncompleteMultipartUpload = &abortXML{DaysAfterInitiation: abort}
		}
		doc.Rules = append(doc.Rules, out)
	}
	return doc
}

func abortDays(r objectstorage.LifecycleRule) *int64 {
	if len(r.AbortMultipartUploadsTransition) == 0 || string(r.AbortMultipartUploadsTransition) == "null" {
		return nil
	}
	var t objectstorage.LifecycleDelete
	if err := json.Unmarshal(r.AbortMultipartUploadsTransition, &t); err != nil || t.Condition.MaxAge == nil {
		return nil
	}
	days := (*t.Condition.MaxAge + secondsPerDay - 1) / secondsPerDay
	return &days
}

func formatLifecycleDate(d string) string {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, d); err == nil {
			return t.UTC().Format(s3Time)
		}
	}
	return d
}

func strPtr(s string) *string { return &s }
