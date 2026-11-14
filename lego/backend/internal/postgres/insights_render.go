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

package postgres

import (
	"cmp"
	"slices"
)

// Canonical Render routes use envelopes and Render's field names. The shorter
// legacy routes and the GraphQL/MCP projections retain their existing views.
type renderProcess struct {
	PID             int32  `json:"pid"`
	DatabaseName    string `json:"databaseName,omitempty"`
	Username        string `json:"username"`
	ApplicationName string `json:"applicationName"`
	ClientAddr      string `json:"clientAddr,omitempty"`
	BackendStart    string `json:"backendStart,omitempty"`
	QueryStart      string `json:"queryStart,omitempty"`
	State           string `json:"state"`
	Query           string `json:"query,omitempty"`
	Masked          bool   `json:"masked,omitempty"`
	WaitEventType   string `json:"waitEventType,omitempty"`
	WaitEvent       string `json:"waitEvent,omitempty"`
	Duration        int32  `json:"duration"`
	IsLeader        bool   `json:"isLeader"`
}
type renderProcesses struct {
	Processes []renderProcess `json:"processes"`
}

func toRenderProcesses(rows []ProcessView) renderProcesses {
	out := renderProcesses{Processes: make([]renderProcess, 0, len(rows))}
	for _, row := range rows {
		out.Processes = append(out.Processes, renderProcess{PID: row.PID, DatabaseName: row.DatabaseName, Username: row.UserName, ApplicationName: row.ApplicationName,
			ClientAddr: row.ClientAddr, BackendStart: row.BackendStart, QueryStart: row.QueryStart, State: row.State, Query: row.Query, Masked: row.Masked,
			WaitEventType: row.WaitEventType, WaitEvent: row.WaitEvent, Duration: row.DurationSeconds, IsLeader: row.IsLeader})
	}
	return out
}

type renderTopQuery struct {
	QueryID             string  `json:"queryId,omitempty"`
	Query               string  `json:"query"`
	Masked              bool    `json:"masked,omitempty"`
	Calls               int64   `json:"calls"`
	TotalTimeMs         float64 `json:"totalTimeMs"`
	MinTimeMs           float64 `json:"minTimeMs"`
	MaxTimeMs           float64 `json:"maxTimeMs"`
	MeanTimeMs          float64 `json:"meanTimeMs"`
	StddevTimeMs        float64 `json:"stddevTimeMs"`
	Rows                int64   `json:"rows"`
	SharedBlocksHit     int64   `json:"sharedBlocksHit"`
	SharedBlocksRead    int64   `json:"sharedBlocksRead"`
	SharedBlocksDirtied int64   `json:"sharedBlocksDirtied"`
	SharedBlocksWritten int64   `json:"sharedBlocksWritten"`
	LocalBlocksHit      int64   `json:"localBlocksHit"`
	LocalBlocksRead     int64   `json:"localBlocksRead"`
	LocalBlocksDirtied  int64   `json:"localBlocksDirtied"`
	LocalBlocksWritten  int64   `json:"localBlocksWritten"`
	TempBlocksRead      int64   `json:"tempBlocksRead"`
	TempBlocksWritten   int64   `json:"tempBlocksWritten"`
}
type renderTopQueries struct {
	TopQueries []renderTopQuery `json:"topQueries"`
}

func toRenderTopQueries(rows []TopQueryView) renderTopQueries {
	out := renderTopQueries{TopQueries: make([]renderTopQuery, 0, len(rows))}
	for _, row := range rows {
		out.TopQueries = append(out.TopQueries, renderTopQuery{QueryID: row.QueryID, Query: row.Query, Masked: row.Masked, Calls: row.Calls,
			TotalTimeMs: row.TotalTimeMs, MinTimeMs: row.MinTimeMs, MaxTimeMs: row.MaxTimeMs, MeanTimeMs: row.MeanTimeMs, StddevTimeMs: row.StddevTimeMs,
			Rows: row.Rows, SharedBlocksHit: row.SharedHitBlks, SharedBlocksRead: row.SharedReadBlks,
			SharedBlocksDirtied: row.SharedDirtiedBlks, SharedBlocksWritten: row.SharedWrittenBlks,
			LocalBlocksHit: row.LocalHitBlks, LocalBlocksRead: row.LocalReadBlks, LocalBlocksDirtied: row.LocalDirtiedBlks, LocalBlocksWritten: row.LocalWrittenBlks,
			TempBlocksRead: row.TempReadBlks, TempBlocksWritten: row.TempWrittenBlks})
	}
	return out
}

type renderSize struct {
	Database string `json:"Database,omitempty"`
	Schema   string `json:"Schema,omitempty"`
	Table    string `json:"Table,omitempty"`
	Index    string `json:"Index,omitempty"`
	Bytes    int64  `json:"Bytes"`
}
type renderSizes struct {
	Sizes []renderSize `json:"sizes"`
}

// toRenderSizes lists the database first, then its tables and indexes
// together, largest first. A table's Bytes excludes its indexes, which have
// rows of their own (w4/634).
func toRenderSizes(view SizesView) renderSizes {
	db := view.Database.Name
	rels := make([]renderSize, 0, len(view.Tables)+len(view.Indexes))
	for _, row := range view.Tables {
		rels = append(rels, renderSize{Database: db, Schema: row.Schema, Table: row.Name, Bytes: row.SizeBytes})
	}
	for _, row := range view.Indexes {
		rels = append(rels, renderSize{Database: db, Schema: row.Schema, Table: row.Table, Index: row.Name, Bytes: row.SizeBytes})
	}
	slices.SortStableFunc(rels, func(a, b renderSize) int { return cmp.Compare(b.Bytes, a.Bytes) })
	return renderSizes{Sizes: append([]renderSize{{Database: db, Bytes: view.Database.SizeBytes}}, rels...)}
}

type renderTableScan struct {
	Database string `json:"Database"`
	Schema   string `json:"Schema"`
	Table    string `json:"Table"`
	Scans    int64  `json:"Scans"`
}
type renderTableScans struct {
	TableScans []renderTableScan `json:"tableScans"`
}

func toRenderTableScans(rows []TableScanView) renderTableScans {
	out := renderTableScans{TableScans: make([]renderTableScan, 0, len(rows))}
	for _, row := range rows {
		out.TableScans = append(out.TableScans, renderTableScan{Database: row.Database, Schema: row.Schema, Table: row.Name, Scans: row.SeqScans})
	}
	return out
}
