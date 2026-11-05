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
	"context"
	"errors"
	"fmt"
	"log"
	"path"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/email"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Failure alert emails (w2/m252). Render emails a workspace when a running
// service becomes unhealthy, a cron run fails, an image pull fails or a
// Blueprint sync fails (render.com/docs/notifications: "Email ✓" on each).
// Recovery is not emailed: Render sends "An unhealthy service becomes
// healthy" to Slack only. Render documents no digest; bex mails the first
// failure at once and folds later ones within the window into one digest, so
// a crash loop cannot flood an inbox while every failure is still reported.
const (
	alertEmailWindow        = time.Hour
	alertEmailDispatchBatch = 200
	alertEmailSendBatch     = 50
	alertEmailClaimLease    = 2 * time.Minute
	alertEmailMaxAttempts   = 8
	alertEmailRetention     = 30 * 24 * time.Hour
	alertEmailSweepEvery    = time.Hour
	alertEmailPollInterval  = 5 * time.Second
	// alertEmailBlueprintLag holds the failed-sync scan back further than the
	// feed's commit lag: completed_at is the sync transaction's start time
	// (now()), and a sync transaction can run for minutes before it commits.
	alertEmailBlueprintLag = 10 * time.Minute
)

// AlertEmailStore is the durable seam; *store.PGStore satisfies it.
type AlertEmailStore interface {
	ListAlertEmailRecipients(context.Context) ([]store.AlertEmailRecipient, error)
	EnsureAlertEmailWatermark(context.Context, string, time.Time) (time.Time, string, error)
	ListWebhookEvents(context.Context, time.Time, string, time.Time, []string, []string, int) ([]store.WebhookEventRow, error)
	EnqueueAlertEmails(context.Context, string, []store.AlertEmailItem, time.Duration, time.Time, string) error
	ClaimDueAlertEmailStreams(context.Context, time.Time, time.Time, int) ([]store.AlertEmailStream, error)
	CompleteAlertEmailStream(context.Context, store.AlertEmailStream, time.Time, time.Duration) (bool, error)
	DropAlertEmailStream(context.Context, store.AlertEmailStream) (bool, error)
	RetryAlertEmailStream(context.Context, store.AlertEmailStream, time.Time) (bool, error)
	ListFailedBlueprintSyncs(context.Context, time.Time, string, time.Time, int) ([]store.FailedBlueprintSync, error)
	SweepAlertEmails(context.Context, time.Time) error
}

// ServiceNotificationPolicy resolves a service's notificationsToSend for a
// failure, folding the legacy notifyOnFail in (see failureNotificationPolicy).
// An unknown service answers "" (default).
type ServiceNotificationPolicy func(ctx context.Context, tenantID, serviceID string) (string, error)

// AlertEmailWorker tails the event feed and failed Blueprint syncs into
// per-recipient alert streams, then mails the streams that fall due.
type AlertEmailWorker struct {
	Store  AlertEmailStore
	Policy ServiceNotificationPolicy
	// Owners resolves each workspace's notification policy (w3/m271); nil
	// applies Render's default (email on, failure only).
	Owners           OwnerPolicyLookup
	Mailer           Mailer
	Identities       EmailLookup
	DashboardBaseURL string
	Clock            func() time.Time
	PollInterval     time.Duration
	// Window is the digest window; 0 => alertEmailWindow.
	Window time.Duration
	// BlueprintLag holds the failed-sync scan back; 0 => alertEmailBlueprintLag.
	BlueprintLag time.Duration

	feedCursor      store.FeedCursor
	blueprintLoaded bool
	blueprintAt     time.Time
	blueprintKey    string
	lastSweep       time.Time
}

// Run polls until cancellation; rows stay durable across failed ticks.
func (w *AlertEmailWorker) Run(ctx context.Context) {
	interval := w.PollInterval
	if interval <= 0 {
		interval = alertEmailPollInterval
	}
	core.Poll(ctx, "notifications: alert email worker", interval, w.RunOnce)
}

func (w *AlertEmailWorker) now() time.Time {
	if w.Clock != nil {
		return w.Clock()
	}
	return time.Now()
}

func (w *AlertEmailWorker) window() time.Duration {
	if w.Window > 0 {
		return w.Window
	}
	return alertEmailWindow
}

func (w *AlertEmailWorker) blueprintLag() time.Duration {
	if w.BlueprintLag > 0 {
		return w.BlueprintLag
	}
	return alertEmailBlueprintLag
}

// RunOnce projects both sources, then sends due streams, then sweeps.
func (w *AlertEmailWorker) RunOnce(ctx context.Context) error {
	if w.Store == nil {
		return errors.New("alert email worker store is required")
	}
	recipients, err := w.Store.ListAlertEmailRecipients(ctx)
	if err != nil {
		return fmt.Errorf("list alert email recipients: %w", err)
	}
	byTenant := map[string][]store.AlertEmailRecipient{}
	for _, r := range recipients {
		byTenant[r.TenantID] = append(byTenant[r.TenantID], r)
	}
	tenants := make([]string, 0, len(byTenant))
	for tenant := range byTenant {
		tenants = append(tenants, tenant)
	}
	sort.Strings(tenants)
	until := w.now().Add(-store.FeedCommitLag)
	return errors.Join(
		w.dispatchFeed(ctx, byTenant, tenants, until),
		w.dispatchBlueprints(ctx, byTenant, w.now().Add(-w.blueprintLag())),
		w.send(ctx),
		w.sweep(ctx),
	)
}

func (w *AlertEmailWorker) dispatchFeed(ctx context.Context, byTenant map[string][]store.AlertEmailRecipient, tenants []string, until time.Time) error {
	ensure := func(ctx context.Context, at time.Time) (time.Time, string, error) {
		return w.Store.EnsureAlertEmailWatermark(ctx, store.AlertEmailFeedWatermark, at)
	}
	if err := w.feedCursor.Load(ctx, ensure, until); err != nil {
		return fmt.Errorf("load alert email feed cursor: %w", err)
	}
	if len(tenants) == 0 {
		return nil
	}
	return store.TailFeed(ctx, &w.feedCursor, store.FeedPass[store.AlertEmailItem]{
		Until:   until,
		Verbs:   []string{},
		Tenants: tenants,
		Limit:   alertEmailDispatchBatch,
		List:    w.Store.ListWebhookEvents,
		Commit: func(ctx context.Context, items []store.AlertEmailItem, at time.Time, key string) error {
			return w.Store.EnqueueAlertEmails(ctx, store.AlertEmailFeedWatermark, items, w.window(), at, key)
		},
		Project: func(ctx context.Context, rows []store.WebhookEventRow) ([]store.AlertEmailItem, error) {
			return w.projectFeed(ctx, rows, byTenant)
		},
	})
}

func (w *AlertEmailWorker) projectFeed(ctx context.Context, rows []store.WebhookEventRow, byTenant map[string][]store.AlertEmailRecipient) ([]store.AlertEmailItem, error) {
	var items []store.AlertEmailItem
	policies := map[string]string{}
	owners := ownerPolicyCache{lookup: w.Owners}
	for _, row := range rows {
		event, detail, ok := classifyAlertEmail(row)
		if !ok || len(byTenant[row.TenantID]) == 0 {
			continue
		}
		// The service is its srv- id (row.AppID, the App CR's app-id label),
		// not the feed's tenant-name composite, which no App carries: keyed on
		// that, every override read as "default" and the email's Events link
		// named no service page.
		serviceID := row.AppID
		if serviceID == "" {
			serviceID = row.ServiceID
		}
		cacheKey := row.TenantID + "/" + serviceID
		service, cached := policies[cacheKey]
		if !cached && w.Policy != nil {
			var err error
			if service, err = w.Policy(ctx, row.TenantID, serviceID); err != nil {
				return nil, fmt.Errorf("resolve notification policy for %s: %w", serviceID, err)
			}
			policies[cacheKey] = service
		}
		owner, err := owners.get(ctx, row.TenantID)
		if err != nil {
			return nil, err
		}
		policy := emailPolicy{owner: owner, service: service}
		name := row.ServiceName
		if name == "" {
			name = row.ServiceID
		}
		for _, r := range byTenant[row.TenantID] {
			if !policy.wants(levelFailure, toggleFailed, alertPrefs(r)) {
				continue
			}
			items = append(items, store.AlertEmailItem{
				TenantID: row.TenantID, Subject: r.Subject, SourceKey: row.Key,
				ResourceID: serviceID, Event: event, ResourceName: name, Detail: detail,
				OccurredAt: row.At,
			})
		}
	}
	return items, nil
}

// classifyAlertEmail maps a feed row onto Render's emailed failure events.
func classifyAlertEmail(row store.WebhookEventRow) (event, detail string, ok bool) {
	if row.Source != store.EventSourceFact {
		return "", "", false
	}
	switch row.FactType {
	case string(store.EventFactServerFailed):
		return store.AlertEmailServerFailed, "", true
	case string(store.EventFactServerHardwareFailure):
		return store.AlertEmailServerFailed, "An instance was lost with the machine it ran on.", true
	case string(store.EventFactImagePullFailed):
		return store.AlertEmailImagePullFailed, "", true
	case string(store.EventFactCronRunEnded):
		if row.Status == store.EventStatusFailed {
			return store.AlertEmailCronFailed, "", true
		}
	}
	return "", "", false
}

// ownerPolicyCache reads each workspace's notification policy once per pass.
type ownerPolicyCache struct {
	lookup OwnerPolicyLookup
	byID   map[string]OwnerPolicy
}

func (c *ownerPolicyCache) get(ctx context.Context, tenantID string) (OwnerPolicy, error) {
	if p, ok := c.byID[tenantID]; ok {
		return p, nil
	}
	p, err := ownerPolicy(ctx, c.lookup, tenantID)
	if err != nil {
		return OwnerPolicy{}, fmt.Errorf("resolve workspace notification policy for %s: %w", tenantID, err)
	}
	if c.byID == nil {
		c.byID = map[string]OwnerPolicy{}
	}
	c.byID[tenantID] = p
	return p, nil
}

// alertPrefs is the failure preference an alert recipient carries.
func alertPrefs(r store.AlertEmailRecipient) memberPrefs { return memberPrefs{failed: r.DeployFailed} }

// failureNotificationPolicy is the policy a service applies to its failure
// notifications: notificationsToSend when set, otherwise the legacy
// notifyOnFail (notify => failure, ignore => none), otherwise default. The
// legacy field only ever spoke about failures, so "notify" is "failure", not
// "all": the two send the same failure mail, but only "all" opts into
// non-failure mail such as a disk back below its threshold.
func failureNotificationPolicy(notificationsToSend, notifyOnFail string) string {
	if notificationsToSend != "" {
		return notificationsToSend
	}
	switch notifyOnFail {
	case "notify":
		return appv1alpha1.NotificationsToSendFailure
	case "ignore":
		return appv1alpha1.NotificationsToSendNone
	}
	return appv1alpha1.NotificationsToSendDefault
}

// dispatchBlueprints projects failed Blueprint syncs. A Blueprint has no
// service override, so the workspace policy and each member's failure
// preference decide.
func (w *AlertEmailWorker) dispatchBlueprints(ctx context.Context, byTenant map[string][]store.AlertEmailRecipient, until time.Time) error {
	if !w.blueprintLoaded {
		at, key, err := w.Store.EnsureAlertEmailWatermark(ctx, store.AlertEmailBlueprintWatermark, until)
		if err != nil {
			return fmt.Errorf("load alert email blueprint cursor: %w", err)
		}
		w.blueprintAt, w.blueprintKey, w.blueprintLoaded = at, key, true
	}
	for {
		failed, err := w.Store.ListFailedBlueprintSyncs(ctx, w.blueprintAt, w.blueprintKey, until, alertEmailDispatchBatch)
		if err != nil {
			return fmt.Errorf("list failed blueprint syncs: %w", err)
		}
		if len(failed) == 0 {
			if until.Sub(w.blueprintAt) > store.DefaultFeedPark {
				if err := w.Store.EnqueueAlertEmails(ctx, store.AlertEmailBlueprintWatermark, nil, w.window(), until, ""); err != nil {
					return err
				}
				w.blueprintAt, w.blueprintKey = until, ""
			}
			return nil
		}
		var items []store.AlertEmailItem
		owners := ownerPolicyCache{lookup: w.Owners}
		for _, f := range failed {
			name := f.BlueprintName
			if name == "" {
				name = id.BlueprintPublicID(f.BlueprintID)
			}
			owner, err := owners.get(ctx, f.TenantID)
			if err != nil {
				return err
			}
			policy := emailPolicy{owner: owner, service: appv1alpha1.NotificationsToSendDefault}
			for _, r := range byTenant[f.TenantID] {
				if !policy.wants(levelFailure, toggleFailed, alertPrefs(r)) {
					continue
				}
				items = append(items, store.AlertEmailItem{
					TenantID: f.TenantID, Subject: r.Subject, SourceKey: "blueprint_sync:" + f.SyncID,
					ResourceID: f.BlueprintID, Event: store.AlertEmailBlueprintSyncFailed,
					ResourceName: name, Detail: f.ErrorMessage, OccurredAt: f.CompletedAt,
				})
			}
		}
		last := failed[len(failed)-1]
		if err := w.Store.EnqueueAlertEmails(ctx, store.AlertEmailBlueprintWatermark, items, w.window(), last.CompletedAt, last.SyncID); err != nil {
			return err
		}
		w.blueprintAt, w.blueprintKey = last.CompletedAt, last.SyncID
		if len(failed) < alertEmailDispatchBatch {
			return nil
		}
	}
}

func (w *AlertEmailWorker) send(ctx context.Context) error {
	if w.Mailer == nil || w.Identities == nil {
		return nil // streams stay due until a mailer is configured
	}
	now := w.now()
	streams, err := w.Store.ClaimDueAlertEmailStreams(ctx, now, now.Add(alertEmailClaimLease), alertEmailSendBatch)
	if err != nil || len(streams) == 0 {
		return err
	}
	subjects := make([]string, 0, len(streams))
	seen := map[string]bool{}
	for _, st := range streams {
		if !seen[st.Subject] {
			seen[st.Subject] = true
			subjects = append(subjects, st.Subject)
		}
	}
	addrs, lookupErr := w.Identities.LookupEmails(ctx, subjects)
	var errs []error
	for _, st := range streams {
		addr := addrs[st.Subject]
		switch {
		case addr == "" && lookupErr != nil:
			errs = append(errs, w.retry(ctx, st, lookupErr))
			continue
		case addr == "":
			log.Printf("notifications: %s alert for %s not emailed to %s (no address)", st.Event, st.ResourceID, st.Subject)
		default:
			subject, msg := alertEmail(st, w.dashboardURL(st))
			if err := w.Mailer.Send(ctx, addr, subject, msg.Text(), msg.HTML()); err != nil {
				if !permanentRejection(err) {
					errs = append(errs, w.retry(ctx, st, err))
					continue
				}
				log.Printf("notifications: %s alert for %s not emailed to %s (address rejected: %v)", st.Event, st.ResourceID, st.Subject, err)
			}
		}
		if _, err := w.Store.CompleteAlertEmailStream(ctx, st, w.now(), w.window()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// retry backs a stream off after a transient failure; past the attempt cap the
// pending failures are dropped (logged) rather than retried forever.
func (w *AlertEmailWorker) retry(ctx context.Context, st store.AlertEmailStream, cause error) error {
	if st.Attempts+1 >= alertEmailMaxAttempts {
		log.Printf("notifications: %s alert for %s to %s dropped after %d attempts: %v", st.Event, st.ResourceID, st.Subject, st.Attempts+1, cause)
		_, err := w.Store.DropAlertEmailStream(ctx, st)
		return err
	}
	_, err := w.Store.RetryAlertEmailStream(ctx, st, w.now().Add(pushRetryDelay(st.Attempts+1)))
	return errors.Join(cause, err)
}

func (w *AlertEmailWorker) sweep(ctx context.Context) error {
	now := w.now()
	if now.Sub(w.lastSweep) < alertEmailSweepEvery {
		return nil
	}
	if err := w.Store.SweepAlertEmails(ctx, now.Add(-alertEmailRetention)); err != nil {
		return err
	}
	w.lastSweep = now
	return nil
}

// dashboardURL is where the email's button points: the service's Events page,
// or the Blueprint's page. Empty when the dashboard URL is unset.
func (w *AlertEmailWorker) dashboardURL(st store.AlertEmailStream) string {
	if w.DashboardBaseURL == "" {
		return ""
	}
	cfg := resourcemeta.Config{DashboardBaseURL: w.DashboardBaseURL}
	if st.Event == store.AlertEmailBlueprintSyncFailed {
		// ResourceID holds the durable blp- id; the page is addressed by the
		// public exs- id the dashboard and API show (w2/m269).
		return cfg.DashboardURL("blueprints", id.BlueprintPublicID(st.ResourceID))
	}
	return cfg.DashboardURL(path.Join("services", st.ResourceID), "events")
}

// alertEmail composes one stream's mail: the single failure, or a digest
// naming how many failures occurred and when.
func alertEmail(st store.AlertEmailStream, url string) (string, email.Message) {
	var title, lead, cta string
	name := st.ResourceName
	switch st.Event {
	case store.AlertEmailServerFailed:
		title, cta = "Service unavailable", "View events"
		lead = fmt.Sprintf("%q became unhealthy: an instance stopped running or failed its health checks.", name)
	case store.AlertEmailCronFailed:
		title, cta = "Cron job failed", "View events"
		lead = fmt.Sprintf("A run of the cron job %q failed.", name)
	case store.AlertEmailImagePullFailed:
		title, cta = "Image pull failed", "View events"
		lead = fmt.Sprintf("%q could not pull its Docker image, so the new version did not start.", name)
	case store.AlertEmailDiskUsageHigh:
		title, cta = "Disk usage high", "View events"
		lead = fmt.Sprintf("A disk on %q crossed its usage threshold. Free up space or increase the disk size before it fills up.", name)
	case store.AlertEmailDiskUsageRecovered:
		title, cta = "Disk usage back to normal", "View events"
		lead = fmt.Sprintf("A disk on %q is back below its usage threshold.", name)
	case store.AlertEmailServiceSuspended:
		title, cta = "Service suspended", "View events"
		lead = fmt.Sprintf("We suspended %q because it repeatedly failed to start and become healthy over %s. "+
			"This is often caused by a configuration error, a bug in the application code, or an undersized instance type. "+
			"Fix the cause, then resume the service from its Settings page.", name, st.Detail)
		st.Detail = ""
	default:
		title, cta = "Blueprint sync failed", "View Blueprint"
		lead = fmt.Sprintf("A sync of the Blueprint %q failed. Its resources may not match the Blueprint file.", name)
	}
	subject := fmt.Sprintf("%s: %s", title, name)
	msg := email.Message{Paragraphs: []string{lead}}
	if detail := strings.TrimSpace(st.Detail); detail != "" {
		msg.Paragraphs = append(msg.Paragraphs, detail)
	}
	if st.PendingCount > 1 {
		subject = fmt.Sprintf("%s: %s (%d times)", title, name, st.PendingCount)
		times := slices.SortedFunc(slices.Values(st.Occurrences), func(a, b time.Time) int { return a.Compare(b) })
		if len(times) > store.AlertEmailOccurrenceCap {
			times = times[len(times)-store.AlertEmailOccurrenceCap:]
		}
		lines := make([]string, 0, len(times)+1)
		for _, at := range times {
			lines = append(lines, at.UTC().Format("2006-01-02 15:04:05 UTC"))
		}
		if omitted := st.PendingCount - len(times); omitted > 0 {
			lines = append(lines, fmt.Sprintf("and %d earlier", omitted))
		}
		since := "your last alert"
		if st.LastMailedAt != nil {
			since = st.LastMailedAt.UTC().Format("2006-01-02 15:04 UTC")
		}
		msg.Paragraphs = append(msg.Paragraphs,
			fmt.Sprintf("This happened %d times since %s. Most recent:\n%s", st.PendingCount, since, strings.Join(lines, "\n")))
	} else if len(st.Occurrences) == 1 {
		msg.Paragraphs = append(msg.Paragraphs, "At "+st.Occurrences[0].UTC().Format("2006-01-02 15:04:05 UTC")+".")
	}
	msg.Title = subject
	if url != "" {
		msg.CTA = &email.CTA{Lead: cta, Label: cta, URL: url}
	}
	return subject, msg
}

// AppNotificationPolicy resolves a service's failure policy from its App CR,
// found by public id within the event's workspace. A deleted service answers
// default, so its last failures still follow each member's preference.
func AppNotificationPolicy(c client.Client) ServiceNotificationPolicy {
	return func(ctx context.Context, tenantID, serviceID string) (string, error) {
		if c == nil || serviceID == "" {
			return appv1alpha1.NotificationsToSendDefault, nil
		}
		var apps appv1alpha1.AppList
		if err := c.List(ctx, &apps, client.MatchingLabels{core.LabelAppID: serviceID, core.LabelTenant: tenantID}); err != nil {
			return "", err
		}
		if len(apps.Items) == 0 {
			return appv1alpha1.NotificationsToSendDefault, nil
		}
		spec := apps.Items[0].Spec
		return failureNotificationPolicy(spec.NotificationsToSend, spec.NotifyOnFail), nil
	}
}

// ServiceAlertStore is what the worker-raised alerts (crash loop, disk
// usage) need; *store.PGStore satisfies it.
type ServiceAlertStore interface {
	ListAlertEmailRecipientsFor(context.Context, string) ([]store.AlertEmailRecipient, error)
	EnqueueAlertEmails(context.Context, string, []store.AlertEmailItem, time.Duration, time.Time, string) error
}

// CrashLoopNotifier emails a workspace that the platform suspended one of its
// services for a crash loop (w2/m253), through the failure-alert queue. The
// suspension is a failure notification: a service's notificationsToSend and
// each member's failure preference apply as for any other failure.
type CrashLoopNotifier struct {
	Store  ServiceAlertStore
	Policy ServiceNotificationPolicy
	Owners OwnerPolicyLookup
	// Window is the suspension window, named in the email.
	Window time.Duration
}

// NotifyCrashLoopSuspended enqueues the alert. Its source key is the service
// and the moment its window started, so a retried or raced suspension mails
// once.
func (n CrashLoopNotifier) NotifyCrashLoopSuspended(ctx context.Context, tenantID, serviceID, name string, failingSince, at time.Time) error {
	return enqueueServiceAlert(ctx, n.Store, n.Policy, n.Owners, tenantID, serviceID, levelFailure, store.AlertEmailItem{
		SourceKey:  "crashloop:" + serviceID + ":" + failingSince.UTC().Format(time.RFC3339),
		ResourceID: serviceID, Event: store.AlertEmailServiceSuspended, ResourceName: name,
		Detail: windowWords(n.Window), OccurredAt: at,
	})
}

// enqueueServiceAlert enqueues template for every member of tenantID whom the
// service's notification level (its override, else the workspace policy)
// admits for an event of level. Under the workspace policy a failure follows
// each member's failure preference; an "all" event has no member preference.
// A worker-raised alert has no feed cursor; template.SourceKey makes a retry
// or a racing replica mail once.
func enqueueServiceAlert(ctx context.Context, st ServiceAlertStore, policyOf ServiceNotificationPolicy, owners OwnerPolicyLookup,
	tenantID, serviceID string, level notifyLevel, template store.AlertEmailItem) error {
	if st == nil {
		return nil
	}
	service := appv1alpha1.NotificationsToSendDefault
	if policyOf != nil {
		var err error
		if service, err = policyOf(ctx, tenantID, serviceID); err != nil {
			return err
		}
	}
	owner, err := ownerPolicy(ctx, owners, tenantID)
	if err != nil {
		return err
	}
	policy, toggle := emailPolicy{owner: owner, service: service}, toggleFailed
	if level == levelAll {
		toggle = toggleNone
	}
	recipients, err := st.ListAlertEmailRecipientsFor(ctx, tenantID)
	if err != nil {
		return err
	}
	var items []store.AlertEmailItem
	for _, r := range recipients {
		if !policy.wants(level, toggle, alertPrefs(r)) {
			continue
		}
		item := template
		item.TenantID, item.Subject = tenantID, r.Subject
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil
	}
	return st.EnqueueAlertEmails(ctx, "", items, alertEmailWindow, time.Time{}, "")
}

// DiskUsageNotifier emails a workspace that a service disk crossed 80% of its
// size, or dropped back below (w2/m255). Render sends the first at the failure
// notification level and the second at the "all notifications" level.
type DiskUsageNotifier struct {
	Store  ServiceAlertStore
	Policy ServiceNotificationPolicy
	Owners OwnerPolicyLookup
}

// NotifyDiskUsage enqueues the alert under the crossing fact's source key.
func (n DiskUsageNotifier) NotifyDiskUsage(ctx context.Context, tenantID, serviceID, name string, high bool,
	d store.DiskUsageDetails, sourceKey string, at time.Time) error {
	event, level := store.AlertEmailDiskUsageHigh, levelFailure
	if !high {
		event, level = store.AlertEmailDiskUsageRecovered, levelAll
	}
	return enqueueServiceAlert(ctx, n.Store, n.Policy, n.Owners, tenantID, serviceID, level, store.AlertEmailItem{
		SourceKey: sourceKey, ResourceID: serviceID, Event: event, ResourceName: name,
		Detail:     diskUsageDetail(d),
		OccurredAt: at,
	})
}

// diskUsageDetail is the disk sentence a disk-usage email carries.
func diskUsageDetail(d store.DiskUsageDetails) string {
	return fmt.Sprintf("The disk %q mounted at %s is %.1f%% full (threshold %.0f%%).", d.DiskName, d.MountPath, d.UsagePercent, d.ThresholdPercent)
}

// windowWords renders a suspension window for the email ("24 hours").
func windowWords(d time.Duration) string {
	switch {
	case d <= 0:
		return "24 hours"
	case d == time.Hour:
		return "1 hour"
	case d%time.Hour == 0:
		return fmt.Sprintf("%d hours", int(d/time.Hour))
	}
	return d.String()
}
