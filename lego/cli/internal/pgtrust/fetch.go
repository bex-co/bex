package pgtrust

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/postgres"
)

// detailLimit caps how much of a refusal body is read and quoted back. The
// endpoint's 503 message is a short sentence; anything larger is not a message
// this launcher should be echoing to a terminal.
const detailLimit = 512

// NewFetcher returns the production reader of a database's server CA.
//
// It goes through the upstream CLI's own authenticated client, so the launcher
// inherits token refresh, the configured control-plane host, and the exact
// request shape upstream would have sent — but decodes the response itself,
// because `serverCaCertificate` is a bex extension field that upstream's
// generated model does not carry.
func NewFetcher() func(ctx context.Context, idOrName string) (string, error) {
	return func(ctx context.Context, idOrName string) (string, error) {
		api, err := client.NewDefaultClient()
		if err != nil {
			return "", err
		}
		id, err := resolveID(ctx, api, idOrName)
		if err != nil {
			return "", err
		}
		resp, err := api.RetrievePostgresConnectionInfo(ctx, id)
		if err != nil {
			return "", err
		}
		defer func() { _ = resp.Body.Close() }()

		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return "", err
		}
		switch resp.StatusCode {
		case http.StatusOK:
			var info struct {
				ServerCACertificate string `json:"serverCaCertificate"`
			}
			if err := json.Unmarshal(body, &info); err != nil {
				return "", err
			}
			// Absent for an internal-only database: no external endpoint, so no
			// verify-full string and nothing to trust.
			return info.ServerCACertificate, nil
		case http.StatusServiceUnavailable:
			return "", fmt.Errorf("%w (%s)", ErrCAUnavailable, refusalDetail(body))
		default:
			return "", fmt.Errorf("connection-info for %s returned HTTP %d", id, resp.StatusCode)
		}
	}
}

// resolveID turns the user's selector into a database ID, reusing upstream's
// own rule: a well-formed ID is used as given, and anything else is a name that
// must resolve to exactly one database in the active workspace.
func resolveID(ctx context.Context, api *client.ClientWithResponses, idOrName string) (string, error) {
	if strings.HasPrefix(idOrName, "dpg-") && (len(idOrName) == 24 || len(idOrName) == 26) {
		return idOrName, nil
	}
	found, err := postgres.NewRepo(api).ListPostgres(ctx, &client.ListPostgresParams{
		Name: &client.NameParam{idOrName},
	})
	if err != nil {
		return "", err
	}
	if len(found) != 1 {
		// Zero or ambiguous: upstream refuses with its own worded error a
		// moment later, and that is the message the user should see.
		return "", fmt.Errorf("%q does not name exactly one Postgres database", idOrName)
	}
	return found[0].Id, nil
}

// refusalDetail extracts the endpoint's own explanation from an error body. The
// API answers both Render's `message` and bex's `error`; either is a
// developer-authored, non-sensitive sentence. An unparseable body degrades to
// its own text rather than hiding why the read failed.
func refusalDetail(body []byte) string {
	var envelope struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	detail := ""
	if err := json.Unmarshal(body, &envelope); err == nil {
		detail = envelope.Message
		if detail == "" {
			detail = envelope.Error
		}
	}
	if detail == "" {
		detail = string(body)
	}
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return "no detail given"
	}
	if len(detail) > detailLimit {
		detail = detail[:detailLimit] + "…"
	}
	return detail
}
