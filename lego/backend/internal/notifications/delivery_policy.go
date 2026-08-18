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

package notifications

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// locationCache memoizes time.LoadLocation. The Go runtime does not: each call
// re-reads and re-parses the zone's tzdata entry. compileDeliveryPolicy runs
// inside Evaluate, which the push worker calls once per recipient per event on
// a 2s dispatch loop, so a full batch would otherwise re-parse the same handful
// of zones thousands of times a minute. A *time.Location is immutable, so
// sharing one across goroutines is safe. Only successes are cached — a bad zone
// is rejected when settings are written, and never memoizing the failure keeps
// a tzdata update able to heal one.
var locationCache sync.Map // string -> *time.Location

func loadLocation(zone string) (*time.Location, error) {
	if cached, ok := locationCache.Load(zone); ok {
		return cached.(*time.Location), nil
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return nil, err
	}
	locationCache.Store(zone, location)
	return location, nil
}

// DeliveryChannel is a user-configurable notification destination. It is
// intentionally closed: native push extends the existing email preference
// model; it does not create an ambient third-party (for example Slack) sink.
type DeliveryChannel string

const (
	DeliveryChannelEmail DeliveryChannel = "email"
	DeliveryChannelPush  DeliveryChannel = "push"
)

// DeliveryEvent is the shared event vocabulary used by every channel.
type DeliveryEvent string

const (
	DeliveryEventDeployStarted      DeliveryEvent = "deploy_started"
	DeliveryEventDeploySucceeded    DeliveryEvent = "deploy_succeeded"
	DeliveryEventDeployFailed       DeliveryEvent = "deploy_failed"
	DeliveryEventServerFailed       DeliveryEvent = "server_failed"
	DeliveryEventServerAvailable    DeliveryEvent = "server_available"
	DeliveryEventServiceSuspended   DeliveryEvent = "service_suspended"
	DeliveryEventServiceResumed     DeliveryEvent = "service_resumed"
	DeliveryEventCronFailed         DeliveryEvent = "cron_failed"
	DeliveryEventAgentNeedsDecision DeliveryEvent = "agent_needs_decision"
	DeliveryEventAgentPRReady       DeliveryEvent = "agent_pr_ready"
	DeliveryEventAgentFailed        DeliveryEvent = "agent_failed"
)

// DeliveryUrgency controls channel filtering and schedule bypass. Critical is
// a bex delivery priority, not an iOS Critical Alerts entitlement.
type DeliveryUrgency string

const (
	DeliveryUrgencyRoutine   DeliveryUrgency = "routine"
	DeliveryUrgencyImportant DeliveryUrgency = "important"
	DeliveryUrgencyCritical  DeliveryUrgency = "critical"
)

// DeliveryWorkspaceRole is the current role read from tenant membership. The
// evaluator consumes it as an authorization input but never derives or grants
// a role itself.
type DeliveryWorkspaceRole string

const (
	DeliveryRoleViewer      DeliveryWorkspaceRole = "viewer"
	DeliveryRoleContributor DeliveryWorkspaceRole = "contributor"
	DeliveryRoleDeveloper   DeliveryWorkspaceRole = "developer"
	DeliveryRoleAdmin       DeliveryWorkspaceRole = "admin"
	DeliveryRoleBilling     DeliveryWorkspaceRole = "billing"
)

// DeliveryClockRange is a local wall-clock interval on the named weekdays.
// Start is inclusive, End is exclusive, and both use HH:MM. A range whose end
// is earlier than its start crosses midnight into the following weekday.
type DeliveryClockRange struct {
	Weekdays []time.Weekday
	Start    string
	End      string
}

// DeliverySchedule combines positive working hours and negative quiet hours.
// With no WorkingHours, all non-quiet time is working time. Deferral is always
// bounded and is evaluated against actual instants, so DST gaps and folds are
// handled by the selected IANA location rather than wall-clock arithmetic.
type DeliverySchedule struct {
	TimeZone     string
	WorkingHours []DeliveryClockRange
	QuietHours   []DeliveryClockRange
	MaxDeferral  time.Duration
}

// DeliveryChannelPolicy is the member's base preference for one channel.
// Missing event keys are disabled, matching a fail-closed event filter.
type DeliveryChannelPolicy struct {
	Enabled        bool
	Events         map[DeliveryEvent]bool
	MinimumUrgency DeliveryUrgency
}

// DeliveryChannelOverride overlays one service's base channel preference.
// Nil scalars inherit; event entries override only the named base entries.
type DeliveryChannelOverride struct {
	Enabled        *bool
	Events         map[DeliveryEvent]bool
	MinimumUrgency *DeliveryUrgency
}

// DeliveryServiceOverride contains per-channel overrides for exactly one
// service. Workspace-wide events simply do not select a service override.
type DeliveryServiceOverride struct {
	Channels map[DeliveryChannel]DeliveryChannelOverride
}

// DeliveryPolicy is a resolved member preference shared by email and push.
// It is deliberately storage-neutral; the existing settings row and future
// push extension can project into this type without coupling policy to SQL.
type DeliveryPolicy struct {
	Channels map[DeliveryChannel]DeliveryChannelPolicy
	Services map[string]DeliveryServiceOverride
	Schedule DeliverySchedule
}

// DeliveryInput contains event and current membership facts. EligibleRoles is
// optional to preserve existing deploy-notification semantics (all workspace
// members); when supplied it restricts delivery to those current roles.
type DeliveryInput struct {
	Channel          DeliveryChannel
	Event            DeliveryEvent
	Urgency          DeliveryUrgency
	WorkspaceID      string
	EventWorkspaceID string
	Subject          string
	WorkspaceRole    DeliveryWorkspaceRole
	EligibleRoles    []DeliveryWorkspaceRole
	ServiceID        string
}

// DeliveryDisposition is the policy's only side effect-free output action.
type DeliveryDisposition string

const (
	DeliverySend  DeliveryDisposition = "send"
	DeliveryDefer DeliveryDisposition = "defer"
	DeliveryDrop  DeliveryDisposition = "drop"
)

// DeliveryReason is stable machine-readable evidence for delivery metrics,
// audit, and tests. It intentionally carries no device token or endpoint.
type DeliveryReason string

const (
	DeliveryReasonReady               DeliveryReason = "ready"
	DeliveryReasonCriticalBypass      DeliveryReason = "critical_bypass"
	DeliveryReasonQuietHours          DeliveryReason = "quiet_hours"
	DeliveryReasonOutsideWorkingHours DeliveryReason = "outside_working_hours"
	DeliveryReasonChannelDisabled     DeliveryReason = "channel_disabled"
	DeliveryReasonEventFiltered       DeliveryReason = "event_filtered"
	DeliveryReasonBelowUrgency        DeliveryReason = "below_urgency"
	DeliveryReasonRoleIneligible      DeliveryReason = "role_ineligible"
	DeliveryReasonWorkspaceMismatch   DeliveryReason = "workspace_mismatch"
	DeliveryReasonDeferralLimit       DeliveryReason = "deferral_limit"
	DeliveryReasonInvalidPolicy       DeliveryReason = "invalid_policy"
)

// DeliveryDecision says when an already-durable notification job may be sent.
// OSCriticalAlert is always false until a separately approved entitlement is
// designed; critical events only bypass bex's own schedule.
type DeliveryDecision struct {
	Disposition      DeliveryDisposition
	Reason           DeliveryReason
	DeliverAt        time.Time
	Urgency          DeliveryUrgency
	BypassedSchedule bool
	OSCriticalAlert  bool
}

// ErrInvalidDeliveryPolicy marks a fail-closed configuration or input error.
var ErrInvalidDeliveryPolicy = errors.New("invalid notification delivery policy")

// MaximumDeliveryDeferral is a hard safety bound even if stored configuration
// asks for a larger delay.
const MaximumDeliveryDeferral = 7 * 24 * time.Hour

// DeliveryPolicyEvaluator injects time for deterministic worker and test use.
// A nil clock is invalid rather than silently consulting wall time.
type DeliveryPolicyEvaluator struct {
	Now func() time.Time
}

type compiledClockRange struct {
	weekdays [7]bool
	start    int
	end      int
}

type compiledDeliveryPolicy struct {
	location *time.Location
	working  []compiledClockRange
	quiet    []compiledClockRange
}

// Evaluate validates the complete policy and input before making a decision.
// Any malformed or unknown value returns a drop decision plus an error.
func (e DeliveryPolicyEvaluator) Evaluate(policy DeliveryPolicy, input DeliveryInput) (DeliveryDecision, error) {
	invalid := DeliveryDecision{Disposition: DeliveryDrop, Reason: DeliveryReasonInvalidPolicy}
	if e.Now == nil {
		return invalid, fmt.Errorf("%w: clock is required", ErrInvalidDeliveryPolicy)
	}
	compiled, err := compileDeliveryPolicy(policy)
	if err != nil {
		return invalid, err
	}
	if err := validateDeliveryInput(input); err != nil {
		return invalid, err
	}
	if input.WorkspaceID != input.EventWorkspaceID {
		return DeliveryDecision{Disposition: DeliveryDrop, Reason: DeliveryReasonWorkspaceMismatch, Urgency: input.Urgency}, nil
	}
	if !roleEligible(input.WorkspaceRole, input.EligibleRoles) {
		return DeliveryDecision{Disposition: DeliveryDrop, Reason: DeliveryReasonRoleIneligible, Urgency: input.Urgency}, nil
	}

	channel, ok := policy.Channels[input.Channel]
	if !ok || !channel.Enabled {
		return DeliveryDecision{Disposition: DeliveryDrop, Reason: DeliveryReasonChannelDisabled, Urgency: input.Urgency}, nil
	}
	if override, ok := serviceChannelOverride(policy, input.ServiceID, input.Channel); ok {
		if override.Enabled != nil {
			channel.Enabled = *override.Enabled
		}
		if override.MinimumUrgency != nil {
			channel.MinimumUrgency = *override.MinimumUrgency
		}
		channel.Events = overlayDeliveryEvents(channel.Events, override.Events)
	}
	if !channel.Enabled {
		return DeliveryDecision{Disposition: DeliveryDrop, Reason: DeliveryReasonChannelDisabled, Urgency: input.Urgency}, nil
	}
	if !channel.Events[input.Event] {
		return DeliveryDecision{Disposition: DeliveryDrop, Reason: DeliveryReasonEventFiltered, Urgency: input.Urgency}, nil
	}
	if urgencyRank(input.Urgency) < urgencyRank(channel.MinimumUrgency) {
		return DeliveryDecision{Disposition: DeliveryDrop, Reason: DeliveryReasonBelowUrgency, Urgency: input.Urgency}, nil
	}

	now := e.Now()
	if now.IsZero() {
		return invalid, fmt.Errorf("%w: clock returned zero time", ErrInvalidDeliveryPolicy)
	}
	if scheduleAllows(now, compiled) {
		return DeliveryDecision{Disposition: DeliverySend, Reason: DeliveryReasonReady, DeliverAt: now, Urgency: input.Urgency}, nil
	}
	if input.Urgency == DeliveryUrgencyCritical {
		return DeliveryDecision{
			Disposition: DeliverySend, Reason: DeliveryReasonCriticalBypass, DeliverAt: now,
			Urgency: input.Urgency, BypassedSchedule: true, OSCriticalAlert: false,
		}, nil
	}
	if next, ok := nextDeliveryTime(now, policy.Schedule.MaxDeferral, compiled); ok {
		return DeliveryDecision{Disposition: DeliveryDefer, Reason: scheduleBlockReason(now, compiled), DeliverAt: next, Urgency: input.Urgency}, nil
	}
	return DeliveryDecision{Disposition: DeliveryDrop, Reason: DeliveryReasonDeferralLimit, Urgency: input.Urgency}, nil
}

func compileDeliveryPolicy(policy DeliveryPolicy) (compiledDeliveryPolicy, error) {
	for channel, cfg := range policy.Channels {
		if !validDeliveryChannel(channel) {
			return compiledDeliveryPolicy{}, invalidPolicyf("unknown channel %q", channel)
		}
		if !validDeliveryUrgency(cfg.MinimumUrgency) {
			return compiledDeliveryPolicy{}, invalidPolicyf("channel %q has unknown minimum urgency %q", channel, cfg.MinimumUrgency)
		}
		if err := validateDeliveryEvents(cfg.Events); err != nil {
			return compiledDeliveryPolicy{}, err
		}
	}
	for serviceID, service := range policy.Services {
		if strings.TrimSpace(serviceID) == "" {
			return compiledDeliveryPolicy{}, invalidPolicyf("empty service override id")
		}
		for channel, override := range service.Channels {
			if !validDeliveryChannel(channel) {
				return compiledDeliveryPolicy{}, invalidPolicyf("service %q has unknown channel %q", serviceID, channel)
			}
			if override.MinimumUrgency != nil && !validDeliveryUrgency(*override.MinimumUrgency) {
				return compiledDeliveryPolicy{}, invalidPolicyf("service %q has unknown minimum urgency %q", serviceID, *override.MinimumUrgency)
			}
			if err := validateDeliveryEvents(override.Events); err != nil {
				return compiledDeliveryPolicy{}, err
			}
		}
	}
	zone := strings.TrimSpace(policy.Schedule.TimeZone)
	if zone == "" || zone == "Local" {
		return compiledDeliveryPolicy{}, invalidPolicyf("IANA timezone is required")
	}
	location, err := loadLocation(zone)
	if err != nil {
		return compiledDeliveryPolicy{}, invalidPolicyf("timezone %q: %v", zone, err)
	}
	if policy.Schedule.MaxDeferral <= 0 || policy.Schedule.MaxDeferral > MaximumDeliveryDeferral {
		return compiledDeliveryPolicy{}, invalidPolicyf("max deferral must be within (0,%s]", MaximumDeliveryDeferral)
	}
	working, err := compileClockRanges(policy.Schedule.WorkingHours)
	if err != nil {
		return compiledDeliveryPolicy{}, err
	}
	quiet, err := compileClockRanges(policy.Schedule.QuietHours)
	if err != nil {
		return compiledDeliveryPolicy{}, err
	}
	return compiledDeliveryPolicy{location: location, working: working, quiet: quiet}, nil
}

func compileClockRanges(ranges []DeliveryClockRange) ([]compiledClockRange, error) {
	compiled := make([]compiledClockRange, 0, len(ranges))
	for _, window := range ranges {
		if len(window.Weekdays) == 0 {
			return nil, invalidPolicyf("clock range has no weekdays")
		}
		start, err := parseClockMinute(window.Start)
		if err != nil {
			return nil, err
		}
		end, err := parseClockMinute(window.End)
		if err != nil {
			return nil, err
		}
		if start == end {
			return nil, invalidPolicyf("clock range start and end are equal")
		}
		item := compiledClockRange{start: start, end: end}
		for _, weekday := range window.Weekdays {
			if weekday < time.Sunday || weekday > time.Saturday {
				return nil, invalidPolicyf("clock range has invalid weekday %d", weekday)
			}
			if item.weekdays[weekday] {
				return nil, invalidPolicyf("clock range repeats weekday %s", weekday)
			}
			item.weekdays[weekday] = true
		}
		compiled = append(compiled, item)
	}
	return compiled, nil
}

func parseClockMinute(value string) (int, error) {
	if len(value) != 5 || value[2] != ':' {
		return 0, invalidPolicyf("clock %q must use HH:MM", value)
	}
	var hour, minute int
	if _, err := fmt.Sscanf(value, "%02d:%02d", &hour, &minute); err != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, invalidPolicyf("clock %q must use a valid 24-hour time", value)
	}
	return hour*60 + minute, nil
}

func validateDeliveryInput(input DeliveryInput) error {
	if !validDeliveryChannel(input.Channel) {
		return invalidPolicyf("unknown input channel %q", input.Channel)
	}
	if !validDeliveryEvent(input.Event) {
		return invalidPolicyf("unknown input event %q", input.Event)
	}
	if !validDeliveryUrgency(input.Urgency) {
		return invalidPolicyf("unknown input urgency %q", input.Urgency)
	}
	if strings.TrimSpace(input.WorkspaceID) == "" || strings.TrimSpace(input.EventWorkspaceID) == "" || strings.TrimSpace(input.Subject) == "" {
		return invalidPolicyf("workspace, event workspace, and subject are required")
	}
	if !validDeliveryRole(input.WorkspaceRole) {
		return invalidPolicyf("unknown workspace role %q", input.WorkspaceRole)
	}
	seenRoles := make(map[DeliveryWorkspaceRole]bool, len(input.EligibleRoles))
	for _, role := range input.EligibleRoles {
		if !validDeliveryRole(role) {
			return invalidPolicyf("unknown eligible workspace role %q", role)
		}
		if seenRoles[role] {
			return invalidPolicyf("duplicate eligible workspace role %q", role)
		}
		seenRoles[role] = true
	}
	return nil
}

func validateDeliveryEvents(events map[DeliveryEvent]bool) error {
	for event := range events {
		if !validDeliveryEvent(event) {
			return invalidPolicyf("unknown event %q", event)
		}
	}
	return nil
}

func validDeliveryChannel(channel DeliveryChannel) bool {
	return channel == DeliveryChannelEmail || channel == DeliveryChannelPush
}

// Validity is derived from orderedDeliveryEvents (service.go) so the closed
// vocabulary is enumerated exactly once per package: a new event is added to
// the constants and that one list.
var validDeliveryEvents = func() map[DeliveryEvent]bool {
	valid := make(map[DeliveryEvent]bool, len(orderedDeliveryEvents))
	for _, event := range orderedDeliveryEvents {
		valid[event] = true
	}
	return valid
}()

func validDeliveryEvent(event DeliveryEvent) bool {
	return validDeliveryEvents[event]
}

// dropRetiredDeliveryEvents strips event values a past build accepted into a
// stored policy but this build no longer enumerates — usage_threshold, whose
// only producer (w8/001) was retired without implementation, is the first such
// value. Read paths (GetPushSettings, storedPushDeliveryPolicy) apply it before
// strict normalization so a stored policy survives the vocabulary shrinking;
// the write path stays strict so retired values cannot re-enter.
func dropRetiredDeliveryEvents(view *PushSettingsView) {
	view.Events = keepValidDeliveryEvents(view.Events)
	for i := range view.ServiceOverrides {
		if view.ServiceOverrides[i].Events != nil {
			filtered := keepValidDeliveryEvents(*view.ServiceOverrides[i].Events)
			view.ServiceOverrides[i].Events = &filtered
		}
	}
}

func keepValidDeliveryEvents(events []DeliveryEvent) []DeliveryEvent {
	kept := make([]DeliveryEvent, 0, len(events))
	for _, event := range events {
		if validDeliveryEvent(event) {
			kept = append(kept, event)
		}
	}
	return kept
}

func validDeliveryUrgency(urgency DeliveryUrgency) bool {
	return urgency == DeliveryUrgencyRoutine || urgency == DeliveryUrgencyImportant || urgency == DeliveryUrgencyCritical
}

func urgencyRank(urgency DeliveryUrgency) int {
	switch urgency {
	case DeliveryUrgencyRoutine:
		return 1
	case DeliveryUrgencyImportant:
		return 2
	case DeliveryUrgencyCritical:
		return 3
	default:
		return 0
	}
}

func validDeliveryRole(role DeliveryWorkspaceRole) bool {
	switch role {
	case DeliveryRoleViewer, DeliveryRoleContributor, DeliveryRoleDeveloper, DeliveryRoleAdmin, DeliveryRoleBilling:
		return true
	default:
		return false
	}
}

func roleEligible(role DeliveryWorkspaceRole, eligible []DeliveryWorkspaceRole) bool {
	if eligible == nil {
		return true
	}
	for _, allowed := range eligible {
		if role == allowed {
			return true
		}
	}
	return false
}

func serviceChannelOverride(policy DeliveryPolicy, serviceID string, channel DeliveryChannel) (DeliveryChannelOverride, bool) {
	if serviceID == "" {
		return DeliveryChannelOverride{}, false
	}
	service, ok := policy.Services[serviceID]
	if !ok {
		return DeliveryChannelOverride{}, false
	}
	override, ok := service.Channels[channel]
	return override, ok
}

func overlayDeliveryEvents(base, override map[DeliveryEvent]bool) map[DeliveryEvent]bool {
	merged := make(map[DeliveryEvent]bool, len(base)+len(override))
	for event, enabled := range base {
		merged[event] = enabled
	}
	for event, enabled := range override {
		merged[event] = enabled
	}
	return merged
}

func scheduleAllows(instant time.Time, policy compiledDeliveryPolicy) bool {
	local := instant.In(policy.location)
	working := len(policy.working) == 0 || clockRangesContain(policy.working, local)
	return working && !clockRangesContain(policy.quiet, local)
}

func scheduleBlockReason(instant time.Time, policy compiledDeliveryPolicy) DeliveryReason {
	local := instant.In(policy.location)
	if clockRangesContain(policy.quiet, local) {
		return DeliveryReasonQuietHours
	}
	return DeliveryReasonOutsideWorkingHours
}

func clockRangesContain(ranges []compiledClockRange, local time.Time) bool {
	minute := local.Hour()*60 + local.Minute()
	weekday := local.Weekday()
	previous := (weekday + 6) % 7
	for _, window := range ranges {
		if window.start < window.end {
			if window.weekdays[weekday] && minute >= window.start && minute < window.end {
				return true
			}
			continue
		}
		if (window.weekdays[weekday] && minute >= window.start) || (window.weekdays[previous] && minute < window.end) {
			return true
		}
	}
	return false
}

// nextDeliveryTime walks real instants, not synthetic local dates. This makes
// nonexistent spring-forward minutes unselectable and checks both occurrences
// of repeated fall-back minutes. Schedule precision is one minute.
func nextDeliveryTime(now time.Time, limit time.Duration, policy compiledDeliveryPolicy) (time.Time, bool) {
	first := now.Truncate(time.Minute).Add(time.Minute)
	deadline := now.Add(limit)
	for candidate := first; !candidate.After(deadline); candidate = candidate.Add(time.Minute) {
		if scheduleAllows(candidate, policy) {
			return candidate, true
		}
	}
	return time.Time{}, false
}

func invalidPolicyf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidDeliveryPolicy, fmt.Sprintf(format, args...))
}
