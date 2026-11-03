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
	if len(req.Tags) > maxTags {
		return core.NewBadRequestError("AGENT_SESSION_TAGS_INVALID", "at most 20 tags", map[string]any{"field": "tags"})
	}
	seen := make(map[string]bool, len(req.Tags))
	tags := make([]string, 0, len(req.Tags))
	for _, tag := range req.Tags {
		if !tagPattern.MatchString(tag) {
			return core.NewBadRequestError("AGENT_SESSION_TAGS_INVALID", "each tag must match [a-z0-9:_-]{1,64}", map[string]any{"field": "tags", "tag": tag})
		}
		if !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	req.Tags = tags
	return nil
}
