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
	"encoding/xml"
	"net/http"
)

// The documents S3 answers with and accepts. The namespace is the one every S3 client
// has always been sent; it is a protocol constant, not a claim to be that service.
const s3Namespace = "http://s3.amazonaws.com/doc/2006-03-01/"

// writeXML answers 200 (or status) with doc.
func writeXML(w http.ResponseWriter, status int, doc any) *Error {
	out, err := xml.Marshal(doc)
	if err != nil {
		return errInternal()
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(out)
	return nil
}

// decodeXML reads a request document, refusing malformed XML and a document with
// anything but one root of the expected name. A request document is small and comes
// from a signed, authorized caller, but its parser still sees only bounded input.
func decodeXML(data []byte, into any) *Error {
	if err := xml.Unmarshal(data, into); err != nil {
		return errMalformedXML("The XML you provided was not well-formed or did not validate against our published schema.")
	}
	return nil
}

type ownerXML struct {
	ID          string `xml:"ID"`
	DisplayName string `xml:"DisplayName"`
}

type listAllMyBucketsXML struct {
	XMLName xml.Name `xml:"ListAllMyBucketsResult"`
	Xmlns   string   `xml:"xmlns,attr"`
	Owner   ownerXML `xml:"Owner"`
	Buckets struct {
		Bucket []bucketXML `xml:"Bucket"`
	} `xml:"Buckets"`
}

type bucketXML struct {
	Name         string `xml:"Name"`
	CreationDate string `xml:"CreationDate"`
}

type locationConstraintXML struct {
	XMLName xml.Name `xml:"LocationConstraint"`
	Xmlns   string   `xml:"xmlns,attr"`
	Value   string   `xml:",chardata"`
}

type createBucketConfigurationXML struct {
	XMLName            xml.Name `xml:"CreateBucketConfiguration"`
	LocationConstraint string   `xml:"LocationConstraint"`
}

type corsConfigurationXML struct {
	XMLName xml.Name      `xml:"CORSConfiguration"`
	Xmlns   string        `xml:"xmlns,attr,omitempty"`
	Rules   []corsRuleXML `xml:"CORSRule"`
}

type corsRuleXML struct {
	ID             string   `xml:"ID,omitempty"`
	AllowedHeaders []string `xml:"AllowedHeader"`
	AllowedMethods []string `xml:"AllowedMethod"`
	AllowedOrigins []string `xml:"AllowedOrigin"`
	ExposeHeaders  []string `xml:"ExposeHeader"`
	MaxAgeSeconds  *int64   `xml:"MaxAgeSeconds"`
}

type lifecycleConfigurationXML struct {
	XMLName xml.Name           `xml:"LifecycleConfiguration"`
	Xmlns   string             `xml:"xmlns,attr,omitempty"`
	Rules   []lifecycleRuleXML `xml:"Rule"`
}

type lifecycleRuleXML struct {
	ID                             string         `xml:"ID,omitempty"`
	Status                         string         `xml:"Status"`
	Prefix                         *string        `xml:"Prefix"`
	Filter                         *filterXML     `xml:"Filter"`
	Expiration                     *expirationXML `xml:"Expiration"`
	AbortIncompleteMultipartUpload *abortXML      `xml:"AbortIncompleteMultipartUpload"`
	// The parts of S3's schema bex does not run; their presence answers 501.
	Transition                  []xml.Name `xml:"Transition"`
	NoncurrentVersionExpiration *struct{}  `xml:"NoncurrentVersionExpiration"`
	NoncurrentVersionTransition []xml.Name `xml:"NoncurrentVersionTransition"`
}

type filterXML struct {
	Prefix                *string   `xml:"Prefix"`
	Tag                   *struct{} `xml:"Tag"`
	And                   *struct{} `xml:"And"`
	ObjectSizeGreaterThan *struct{} `xml:"ObjectSizeGreaterThan"`
	ObjectSizeLessThan    *struct{} `xml:"ObjectSizeLessThan"`
}

type expirationXML struct {
	Days                      *int64 `xml:"Days"`
	Date                      string `xml:"Date,omitempty"`
	ExpiredObjectDeleteMarker *bool  `xml:"ExpiredObjectDeleteMarker"`
}

type abortXML struct {
	DaysAfterInitiation *int64 `xml:"DaysAfterInitiation"`
}
