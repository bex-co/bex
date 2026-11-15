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

// Package objectstorage is bex's object store (ADR112): workspace buckets that
// are rows in Postgres over one shared Wasabi bucket per region, served to the
// Render CLI and to stock wrangler.
//
// Everything that decides what a region, a name, a key or a cap is lives here
// as a constant. An environment variable carries a credential and nothing else
// (ADR112 D9), so this package reads none.
package objectstorage

import (
	"errors"
	"fmt"
	"strings"
)

// Region is a bex object storage region: where a bucket's objects live.
type Region string

const (
	Oregon    Region = "oregon"
	Frankfurt Region = "frankfurt"
)

// DefaultRegion is where a bucket created without a location hint goes.
const DefaultRegion = Oregon

// Regions lists every region bex offers, in the order the docs name them.
var Regions = []Region{Oregon, Frankfurt}

// ErrRegionNotOffered is the error behind every region or location hint bex
// does not serve; callers answer it with HTTP 400 (ADR112 D1).
var ErrRegionNotOffered = errors.New("region is not offered")

// RegionNotOfferedError names the value that is not served.
type RegionNotOfferedError struct{ Value string }

func (e *RegionNotOfferedError) Error() string {
	return fmt.Sprintf("region %q is not offered; bex object storage has %s", e.Value, joinRegions())
}

func (e *RegionNotOfferedError) Unwrap() error { return ErrRegionNotOffered }

func joinRegions() string {
	names := make([]string, len(Regions))
	for i, r := range Regions {
		names[i] = string(r)
	}
	return strings.Join(names, " and ")
}

// wasabiRegions maps each region to the Wasabi region that holds it.
var wasabiRegions = map[Region]string{
	Oregon:    "us-west-1",
	Frankfurt: "eu-central-2",
}

// WasabiRegion is the Wasabi region (S3 signing region) of r.
func (r Region) WasabiRegion() string { return wasabiRegions[r] }

// Bucket is the shared Wasabi bucket every workspace's objects of region r
// live in. Wasabi bucket names are global, so provisioning may find the name
// taken; the committed IAM policies name these buckets literally.
func (r Region) Bucket() string { return "bex-objects-" + string(r) }

// Endpoint is the Wasabi S3 service URL of r.
func (r Region) Endpoint() string {
	return "https://s3." + r.WasabiRegion() + ".wasabisys.com"
}

// ParseRegion reads a region as Render's object routes carry it in the path.
func ParseRegion(value string) (Region, error) {
	for _, r := range Regions {
		if string(r) == value {
			return r, nil
		}
	}
	return "", &RegionNotOfferedError{Value: value}
}

// RegionFromHint reads the placement a Cloudflare bucket create asks for: a
// location hint (`wnam`, `weur`, ...) and a jurisdiction (the cf-r2-jurisdiction
// header, `eu`, `fedramp`, `default`). No hint and the hint `wnam` give oregon;
// `weur` and the jurisdiction `eu` give frankfurt; every other value is not
// offered. Cloudflare calls a hint best effort; bex's region is exact.
func RegionFromHint(hint, jurisdiction string) (Region, error) {
	hint = strings.ToLower(strings.TrimSpace(hint))
	jurisdiction = strings.ToLower(strings.TrimSpace(jurisdiction))
	switch jurisdiction {
	case "", "default":
		switch hint {
		case "", "wnam":
			return Oregon, nil
		case "weur":
			return Frankfurt, nil
		}
		return "", &RegionNotOfferedError{Value: hint}
	case "eu":
		if hint == "" || hint == "weur" {
			return Frankfurt, nil
		}
		return "", &RegionNotOfferedError{Value: hint}
	}
	return "", &RegionNotOfferedError{Value: jurisdiction}
}
