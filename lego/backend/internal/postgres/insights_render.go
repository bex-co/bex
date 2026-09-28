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

// Canonical Render routes use envelopes and Render's field names. The shorter
// legacy routes and the GraphQL/MCP projections retain their existing views.
type renderProcess struct {
	PID             int32  `json:"pid"`
	Username        string `json:"username"`
	ApplicationName string `json:"applicationName"`
	State           string `json:"state"`
	Query           string `json:"query,omitempty"`
	Masked          bool   `json:"masked,omitempty"`
	WaitEventType   string `json:"waitEventType,omitempty"`
	WaitEvent       string `json:"waitEvent,omitempty"`
	Duration        int32  `json:"duration"`
}
type renderProcesses struct {
	Processes []renderProcess `json:"processes"`
}

func toRenderProcesses(rows []ProcessView) renderProcesses {
	out := renderProcesses{Processes: make([]renderProcess, 0, len(rows))}
	for _, row := range rows {
		out.Processes = append(out.Processes, renderProcess{PID: row.PID, Username: row.UserName, ApplicationName: row.ApplicationName, State: row.State, Query: row.Query, Masked: row.Masked, WaitEventType: row.WaitEventType, WaitEvent: row.WaitEvent, Duration: row.DurationSeconds})
	}
	return out
}

type renderTopQuery struct {
	Query            string  `json:"query"`
	Masked           bool    `json:"masked,omitempty"`
	Calls            int64   `json:"calls"`
	TotalTimeMs      float64 `json:"totalTimeMs"`
	MeanTimeMs       float64 `json:"meanTimeMs"`
	Rows             int64   `json:"rows"`
	SharedBlocksHit  int64   `json:"sharedBlocksHit"`
	SharedBlocksRead int64   `json:"sharedBlocksRead"`
}
type renderTopQueries struct {
	TopQueries []renderTopQuery `json:"topQueries"`
}

func toRenderTopQueries(rows []TopQueryView) renderTopQueries {
	out := renderTopQueries{TopQueries: make([]renderTopQuery, 0, len(rows))}
	for _, row := range rows {
		out.TopQueries = append(out.TopQueries, renderTopQuery{Query: row.Query, Masked: row.Masked, Calls: row.Calls, TotalTimeMs: row.TotalTimeMs, MeanTimeMs: row.MeanTimeMs, Rows: row.Rows, SharedBlocksHit: row.SharedHitBlks, SharedBlocksRead: row.SharedReadBlks})
	}
	return out
}

type renderSize struct {
	Database string `json:"Database,omitempty"`
	Schema   string `json:"Schema,omitempty"`
	Table    string `json:"Table,omitempty"`
	Bytes    int64  `json:"Bytes"`
}
type renderSizes struct {
	Sizes []renderSize `json:"sizes"`
}

func toRenderSizes(view SizesView) renderSizes {
	out := renderSizes{Sizes: make([]renderSize, 0, len(view.Tables)+1)}
	out.Sizes = append(out.Sizes, renderSize{Database: view.Database.Name, Bytes: view.Database.SizeBytes})
	for _, row := range view.Tables {
		out.Sizes = append(out.Sizes, renderSize{Database: view.Database.Name, Schema: row.Schema, Table: row.Name, Bytes: row.SizeBytes})
	}
	return out
}

type renderTableScan struct {
	Schema string `json:"Schema"`
	Table  string `json:"Table"`
	Scans  int64  `json:"Scans"`
}
type renderTableScans struct {
	TableScans []renderTableScan `json:"tableScans"`
}

func toRenderTableScans(rows []TableScanView) renderTableScans {
	out := renderTableScans{TableScans: make([]renderTableScan, 0, len(rows))}
	for _, row := range rows {
		out.TableScans = append(out.TableScans, renderTableScan{Schema: row.Schema, Table: row.Name, Scans: row.SeqScans})
	}
	return out
}
