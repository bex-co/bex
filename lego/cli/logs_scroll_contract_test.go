package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/logs"
	"github.com/render-oss/cli/pkg/tui/views"
)

// logsScrollRequest is the request the pinned client's log viewer sends to
// load an older page, as lego/cli/testdata/logs-scroll-request.json records
// it. lego/backend/internal/api/logs_scroll_contract_test.go replays the same
// query through bex-api's composed handler, Render contract gate included,
// and needs a 200: the gate once refused the CLI's limit=1000 with a 400, so
// scrolling up in `render logs` failed on bex (w3/m475).
type logsScrollRequest struct {
	PaginationLogLimit int    `json:"paginationLogLimit"`
	Path               string `json:"path"`
	Query              string `json:"query"`
}

// TestLogsScrollRequestWire builds the scroll request the way the pinned
// viewer does (pkg/tui/views/logview.go SetLoadMoreFunc: the first page's
// input, the cursors it returned, and Limit = logs.PaginationLogLimit), then
// lets the pinned LogLoader and generated client put it on the wire. Reading
// the constant, not a literal 1000, makes a CLI pin bump that changes the page
// size (w22/m444's v2.37.0 move, for one) fail here until the golden and the
// backend replay agree again. Regenerate with BEX_UPDATE_GOLDEN=1.
func TestLogsScrollRequestWire(t *testing.T) {
	start := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 11, 18, 9, 41, 7, 123456788, time.UTC)

	first := views.LogInput{ResourceIDs: []string{"srv-d4scrollcontract0000"}, Direction: "backward"}
	scroll := first
	scroll.StartTime = &command.TimeOrRelative{T: &start}
	scroll.EndTime = &command.TimeOrRelative{T: &end}
	scroll.Limit = logs.PaginationLogLimit

	// A local loader skips the CLI config, which is where ToParam reads the
	// workspace; the signed-in viewer sends it as ownerId.
	params, err := views.NewLocalLogLoader(nil).ToParam(context.Background(), scroll)
	if err != nil {
		t.Fatal(err)
	}
	params.OwnerId = "tea-d4scrollcontract0000"
	req, err := client.NewListLogsRequest("https://api.bex.co/v1/", params)
	if err != nil {
		t.Fatal(err)
	}
	got := logsScrollRequest{PaginationLogLimit: logs.PaginationLogLimit, Path: req.URL.Path, Query: req.URL.RawQuery}

	golden := filepath.Join("testdata", "logs-scroll-request.json")
	if os.Getenv("BEX_UPDATE_GOLDEN") == "1" {
		var out bytes.Buffer
		encoder := json.NewEncoder(&out)
		encoder.SetEscapeHTML(false) // keep the query's & readable
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(got); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, out.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	var want logsScrollRequest
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("the pinned viewer's scroll request is\n%+v\nbut %s records\n%+v\nregenerate it with BEX_UPDATE_GOLDEN=1 and rerun the backend replay", got, golden, want)
	}
}
