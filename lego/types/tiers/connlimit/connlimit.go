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

// Package connlimit holds the connection limit of every managed datastore
// plan: Render's limit for each Key Value plan, each Postgres plan's
// max_connections, and the managed PgBouncer's client limit. Package tiers
// builds its Postgres and Valkey ladders on it, and refuses to load a
// datastore plan this package gives no limit.
//
// It lives apart from the rest of the catalog for the SNI proxies (w6/154,
// docs/ADR021-keyvalue-management.md § 10). kv-sni-proxy and pg-sni-proxy size
// each route from these figures, and a Go binary changes with any source edit
// in a package it links, because its build ID hashes every source file. While
// they linked tiers, every compute plan or serving pool edit moved both pins,
// and each move closed every external client still open an hour after the old
// pod's SIGTERM. They link this package and none of tiers: an edit here rolls
// both, as a figure their routes read must, and an edit anywhere in tiers
// leaves both byte-identical. Keep it to what the routes read.
package connlimit

import (
	"maps"
	"slices"
)

// Ladder is one datastore family's connection limits, by its CRD's spec.plan
// id.
type Ladder struct {
	defaultPlan string
	limits      map[string]int32
}

// Valkey is Render's connection limit for each Key Value plan
// (render.com/docs/compute-plans#render-key-value-plans, read 2026-11-17): the
// client connections a store accepts before the next is refused with Valkey's
// "ERR max number of clients reached" (w6/m456, ADR021 § 9).
var Valkey = Ladder{defaultPlan: "free", limits: map[string]int32{
	"free":      50,
	"starter":   250,
	"standard":  1000,
	"3g":        3000,
	"pro":       5000,
	"pro_plus":  10000,
	"pro_max":   20000,
	"pro_ultra": 40000,
	"50g":       50000,
}}

// Postgres is Render's connection limit for each Postgres plan
// (render.com/docs/compute-plans, read 2026-11-17: 100 up to 4 GB, 200 at 8 GB,
// 400 at 16 GB, 500 from 32 GB): PostgreSQL's max_connections, which the
// operator sets on the server (w6/134), and the primary's and each replica's
// route admit up to twice it (w6/m457, ADR009).
var Postgres = Ladder{defaultPlan: "free", limits: map[string]int32{
	"free":        100,
	"basic-256mb": 100,
	"basic-1gb":   100,
	"1c-2g":       100,
	"pro-4gb":     100,
	"basic-4gb":   100,
}}

// PostgresPoolerMaxClientConn is the client connections a database's managed
// PgBouncer accepts on its pooled URL, Render's max_client_conn
// (render.com/docs/postgresql-connection-pooling, read 2026-11-17). It is the
// pooled route's connection limit at the external front door (w6/m457).
const PostgresPoolerMaxClientConn = 30000

// Limit is plan's connection limit, and false for a plan the ladder lacks.
func (l Ladder) Limit(plan string) (int32, bool) {
	limit, ok := l.limits[plan]
	return limit, ok
}

// Resolve is the connection limit of the plan a datastore runs on for its
// spec.plan: that plan's, or the default plan's for an empty or unknown id,
// the rule of tiers' Resolve, by which the operator sizes the server.
func (l Ladder) Resolve(plan string) int32 {
	if limit, ok := l.limits[plan]; ok {
		return limit
	}
	return l.limits[l.defaultPlan]
}

// DefaultPlan is the plan an empty or unknown spec.plan runs on. tiers refuses
// a catalog whose default for the family differs.
func (l Ladder) DefaultPlan() string { return l.defaultPlan }

// Plans is every plan the ladder names, sorted.
func (l Ladder) Plans() []string { return slices.Sorted(maps.Keys(l.limits)) }
