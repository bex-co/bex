package agentsessions

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

const (
	maxTitleChars = 200
	maxTags       = 20
	// tagsInvalidCode is the 400 for any tag input create or ReplaceTags refuses.
	tagsInvalidCode = "AGENT_SESSION_TAGS_INVALID"
)

var tagPattern = regexp.MustCompile(`^[a-z0-9:_-]{1,64}$`)

// validateTitleAndTags normalizes the create-time display title and tags
// (ADR094 D3). An empty title becomes the task's first line. Tags are unique,
// at most 20, each [a-z0-9:_-]{1,64}; a bad one is a named 400, never dropped.
func validateTitleAndTags(req *CreateRequest) error {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		task, _, _ := strings.Cut(strings.TrimSpace(req.AgentConfig.Task), "\n")
		title = strings.TrimSpace(task)
	}
	if utf8.RuneCountInString(title) > maxTitleChars {
		if req.Title != "" {
			return core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "title exceeds 200 characters", map[string]any{"field": "title"})
		}
		title = string([]rune(title)[:maxTitleChars]) // a derived title is clipped, never refused
	}
	req.Title = title
	tags, err := normalizeTags(req.Tags)
	if err != nil {
		return err
	}
	req.Tags = tags
	return nil
}

// normalizeTags applies the tag rules shared by create and ReplaceTags: at
// most 20, each [a-z0-9:_-]{1,64}, duplicates collapsed in first-seen order.
func normalizeTags(in []string) ([]string, error) {
	if len(in) > maxTags {
		return nil, core.NewBadRequestError(tagsInvalidCode, "at most 20 tags", map[string]any{"field": "tags"})
	}
	seen := make(map[string]bool, len(in))
	tags := make([]string, 0, len(in))
	for _, tag := range in {
		if !tagPattern.MatchString(tag) {
			return nil, core.NewBadRequestError(tagsInvalidCode, "each tag must match [a-z0-9:_-]{1,64}", map[string]any{"field": "tags", "tag": tag})
		}
		if !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

// replaceTagsRequest is the REST body of PUT /v1/agent-sessions/{id}/tags,
// Devin's {"tags": [...]}. Tags is required: an empty list clears them, a
// missing key is a 400 rather than a silent clear.
type replaceTagsRequest struct {
	Tags *[]string `json:"tags"`
}

func errTagsRequired() error {
	return core.NewBadRequestError(tagsInvalidCode, "tags is required; send [] to clear them", map[string]any{"field": "tags"})
}
