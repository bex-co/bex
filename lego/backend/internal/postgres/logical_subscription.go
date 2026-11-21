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
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Logical replication as a subscriber (w34/m542; docs/ADR009 § Logical
// replication › Subscribers). Render enables subscribers on request like
// publishers, and the user then creates "a subscription on each subscriber"
// (render.com/docs/postgresql-logical-replication, read 2026-11-20). From
// PostgreSQL 16 that takes pg_create_subscription and CREATE on the
// database, which the subscriber role gets; on 13-15 only a superuser may,
// so bex creates each requested subscription as CloudNativePG's superuser,
// as Render's support team can.

// LogicalSubscriptionRequest is one subscription bex creates on PostgreSQL
// 13-15: its name, the publisher's connection URL (with the password the
// subscription authenticates with) and the publication it follows.
type LogicalSubscriptionRequest struct {
	Name         string `json:"name"`
	PublisherURL string `json:"publisherUrl,omitempty"`
	Publication  string `json:"publication,omitempty"`
}

// parseSubscriptionRequest checks req and returns the spec entry (without
// its password Secret) and the password. No message repeats the URL: it
// carries the password.
func parseSubscriptionRequest(req LogicalSubscriptionRequest, params map[string]any) (appv1alpha1.DatabaseLogicalSubscription, string, error) {
	invalid := func(message string) (appv1alpha1.DatabaseLogicalSubscription, string, error) {
		return appv1alpha1.DatabaseLogicalSubscription{}, "", core.NewBadRequestError(logicalReplicationInvalidRequestCode, message, params)
	}
	params["subscription"] = req.Name
	if !appv1alpha1.ValidPostgresIdentifier(req.Name) {
		return invalid(fmt.Sprintf("subscription name %q is not a lowercase PostgreSQL identifier (a-z, 0-9 and _, at most 63 bytes)", req.Name))
	}
	if req.Publication == "" || len(req.Publication) > 63 || strings.ContainsRune(req.Publication, 0) {
		return invalid("publication must name the publication to follow (at most 63 bytes)")
	}
	u, err := url.Parse(req.PublisherURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Opaque != "" || u.Fragment != "" {
		return invalid("publisherUrl must be a postgresql://user:password@host:port/database URL")
	}
	sub := appv1alpha1.DatabaseLogicalSubscription{Name: req.Name, Publication: req.Publication, Host: u.Hostname(), User: u.User.Username()}
	password, hasPassword := u.User.Password()
	switch {
	case sub.Host == "" || len(sub.Host) > 253:
		return invalid("publisherUrl must name the publisher's host")
	case sub.User == "" || len(sub.User) > 63 || !hasPassword || password == "":
		return invalid("publisherUrl must carry the user and the password the subscription authenticates with")
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return invalid("publisherUrl's port is not a TCP port")
		}
		sub.Port = int32(n)
	}
	if sub.DBName = strings.TrimPrefix(u.Path, "/"); sub.DBName == "" || len(sub.DBName) > 63 || strings.Contains(sub.DBName, "/") {
		return invalid("publisherUrl must name the publisher's database")
	}
	for key, values := range u.Query() {
		if key != "sslmode" || len(values) != 1 || !slices.Contains(appv1alpha1.LogicalSubscriptionSSLModes, values[0]) {
			return invalid("publisherUrl may carry only sslmode, one of " + strings.Join(appv1alpha1.LogicalSubscriptionSSLModes, ", "))
		}
		sub.SSLMode = values[0]
	}
	return sub, password, nil
}

// checkLogicalReplicationSQL applies the checks that need the database
// itself: the publisher's schemas and publication name, the subscriber form
// for its PostgreSQL major, and a subscriber role that would be replaced
// while it still owns subscriptions.
func checkLogicalReplicationSQL(ctx context.Context, uri string, change *logicalReplicationChange) error {
	conn, err := connectForLogicalReplication(ctx, uri)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	if change.publisher {
		if err := checkPublisherSQL(ctx, conn, change.next); err != nil {
			return err
		}
	}
	if !change.subscriberRole && change.subscription < 0 {
		return nil
	}
	major, err := serverMajor(ctx, conn)
	if err != nil {
		return err
	}
	params := map[string]any{"postgresMajor": major}
	switch {
	case change.subscriberRole && major < appv1alpha1.PgCreateSubscriptionMajor:
		return core.NewBadRequestError(LogicalReplicationSubscriberFormCode, fmt.Sprintf(
			"PostgreSQL %d lets only a superuser create a subscription, so no role of the database can; name the subscription instead (its name, the publisher's URL and the publication) and bex creates it", major), params)
	case change.subscription >= 0 && major >= appv1alpha1.PgCreateSubscriptionMajor:
		return core.NewBadRequestError(LogicalReplicationSubscriberFormCode, fmt.Sprintf(
			"on PostgreSQL %d the database's own role creates its subscriptions; enable a subscriber role instead", major), params)
	}
	if previous := change.previous; previous != nil && previous.SubscriberRole != "" && previous.SubscriberRole != change.next.SubscriberRole {
		return refuseOwnedSubscriptionsOn(ctx, conn, previous.SubscriberRole)
	}
	if change.recovered && !change.previous.Subscribes() {
		return refuseRecoveredSubscriptions(ctx, conn)
	}
	return nil
}

// refuseRecoveredSubscriptions refuses the first subscriber enable of a
// recovered copy while its catalog holds subscriptions, which can only be its
// source's, restored from the backup (w34/m543). The operator runs the copy
// with no logical replication worker until it subscribes; the enable lifts
// that, and each of these would then connect to its source's publisher and
// compete with the source for the slot. Detached first, dropping one leaves
// the publisher's slot to the source.
func refuseRecoveredSubscriptions(ctx context.Context, conn *pgx.Conn) error {
	var carried []string
	if err := conn.QueryRow(ctx,
		`SELECT coalesce(array_agg(d.datname || '.' || s.subname ORDER BY d.datname, s.subname), '{}')
		   FROM pg_catalog.pg_subscription s JOIN pg_catalog.pg_database d ON d.oid = s.subdbid`).Scan(&carried); err != nil {
		return mapPGError(err)
	}
	if len(carried) == 0 {
		return nil
	}
	return core.NewConflictError(LogicalReplicationSubscriptionsExistCode, fmt.Sprintf(
		"the database was recovered from another's backup and still holds its subscriptions %s, which would compete with that database for its publisher's slots; detach and drop each first (ALTER SUBSCRIPTION … DISABLE, ALTER SUBSCRIPTION … SET (slot_name = NONE), DROP SUBSCRIPTION …)",
		strings.Join(carried, ", ")), map[string]any{"subscriptions": carried})
}

func serverMajor(ctx context.Context, conn *pgx.Conn) (int, error) {
	var major int
	err := conn.QueryRow(ctx, `SELECT current_setting('server_version_num')::int / 10000`).Scan(&major)
	return major, mapPGError(err)
}

// refuseOwnedSubscriptions refuses taking pg_create_subscription from role
// while it owns subscriptions in any of the instance's databases: they would
// keep running, owned by a role no longer enabled. The user drops them first.
func refuseOwnedSubscriptions(ctx context.Context, uri, role string) error {
	conn, err := connectForLogicalReplication(ctx, uri)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	return refuseOwnedSubscriptionsOn(ctx, conn, role)
}

func refuseOwnedSubscriptionsOn(ctx context.Context, conn *pgx.Conn, role string) error {
	// pg_subscription is shared by every database; its columns but the
	// connection string are readable by every role.
	var owned []string
	if err := conn.QueryRow(ctx,
		`SELECT coalesce(array_agg(d.datname || '.' || s.subname ORDER BY d.datname, s.subname), '{}')
		   FROM pg_catalog.pg_subscription s
		   JOIN pg_catalog.pg_database d ON d.oid = s.subdbid
		   JOIN pg_catalog.pg_roles r ON r.oid = s.subowner
		  WHERE r.rolname = $1`, role).Scan(&owned); err != nil {
		return mapPGError(err)
	}
	if len(owned) == 0 {
		return nil
	}
	return core.NewConflictError(LogicalReplicationSubscriptionsExistCode,
		fmt.Sprintf("%s still owns the subscriptions %s; drop them (DROP SUBSCRIPTION) before disabling the subscriber", role, strings.Join(owned, ", ")),
		map[string]any{"subscriberRole": role, "subscriptions": owned})
}

// grantSubscriberCreate gives role CREATE on the database, which CREATE
// SUBSCRIPTION needs beside pg_create_subscription, when it lacks it. It
// runs as the database's default user, who owns the database through its
// owner role. The owner and owner-privileged credentials have it already.
func grantSubscriberCreate(ctx context.Context, uri, role string) error {
	conn, err := connectForLogicalReplication(ctx, uri)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	var dbname string
	var has bool
	if err := conn.QueryRow(ctx, `SELECT current_database(), pg_catalog.has_database_privilege($1, current_database(), 'CREATE')`, role).Scan(&dbname, &has); err != nil {
		return mapPGError(err)
	}
	if has {
		return nil
	}
	if _, err := conn.Exec(ctx, "GRANT CREATE ON DATABASE "+pgx.Identifier{dbname}.Sanitize()+" TO "+pgx.Identifier{role}.Sanitize()); err != nil {
		return fmt.Errorf("granting %s CREATE on the database: %w", role, mapPGError(err))
	}
	return nil
}

// logicalReplicationState is what PostgreSQL reports.
type logicalReplicationState struct {
	major                int
	replicates           bool     // the publisher role holds REPLICATION
	allTables            bool     // the all-tables publication exists as FOR ALL TABLES
	createsSubscriptions bool     // the subscriber role is a member of pg_create_subscription
	subscriptions        []string // the database's subscriptions, sorted
}

// readLogicalReplicationState reads the state for publisher and subscriber
// (either may be empty, which reads false).
func readLogicalReplicationState(ctx context.Context, uri, publisher, subscriber string) (logicalReplicationState, error) {
	var st logicalReplicationState
	conn, err := connectForLogicalReplication(ctx, uri)
	if err != nil {
		return st, err
	}
	defer func() { _ = conn.Close(ctx) }()
	err = conn.QueryRow(ctx,
		`SELECT current_setting('server_version_num')::int / 10000,
		        coalesce((SELECT rolreplication FROM pg_catalog.pg_roles WHERE rolname = $1), false),
		        coalesce((SELECT puballtables FROM pg_catalog.pg_publication WHERE pubname = $2), false),
		        coalesce((SELECT pg_catalog.pg_has_role(r.oid, c.oid, 'MEMBER') FROM pg_catalog.pg_roles r, pg_catalog.pg_roles c
		                   WHERE r.rolname = $3 AND c.rolname = 'pg_create_subscription'), false),
		        coalesce((SELECT array_agg(subname::text ORDER BY subname) FROM pg_catalog.pg_subscription
		                   WHERE subdbid = (SELECT oid FROM pg_catalog.pg_database WHERE datname = current_database())), '{}')`,
		publisher, appv1alpha1.LogicalReplicationAllTablesPublication, subscriber).
		Scan(&st.major, &st.replicates, &st.allTables, &st.createsSubscriptions, &st.subscriptions)
	return st, mapPGError(err)
}

// storeSubscriptionPassword puts password in a Secret of entry's own and
// names it in entry. The same password again keeps the Secret it is already
// in, so a repeated request writes nothing. A new password gets a new
// Secret, so the Cluster's externalClusters entry changes and CNPG rewrites
// the passfile; the replaced one is deleted once the spec no longer names it
// (removedSubscriptionSecrets).
func (s *Service) storeSubscriptionPassword(ctx context.Context, d *appv1alpha1.Database, entry *appv1alpha1.DatabaseLogicalSubscription, password string) error {
	if entry.PasswordSecret != "" {
		var sec corev1.Secret
		err := s.Client.Get(ctx, client.ObjectKey{Namespace: d.Namespace, Name: entry.PasswordSecret}, &sec)
		if err == nil && string(sec.Data["password"]) == password {
			return nil
		}
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return err
	}
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.LogicalSubscriptionObjectName(d.Name, entry.Name) + "-" + hex.EncodeToString(suffix), Namespace: d.Namespace},
		Type:       corev1.SecretTypeBasicAuth,
		Data:       map[string][]byte{"username": []byte(entry.User), "password": []byte(password)},
	}
	// Collected with the Database.
	if err := controllerutil.SetOwnerReference(d, sec, s.Client.Scheme()); err != nil {
		return err
	}
	if err := s.Client.Create(ctx, sec); err != nil {
		return err
	}
	entry.PasswordSecret = sec.Name
	return nil
}

// removedSubscriptionSecrets are the password Secrets of the subscriptions
// previous has and next does not.
func removedSubscriptionSecrets(previous, next *appv1alpha1.DatabaseLogicalReplication) []string {
	if previous == nil {
		return nil
	}
	var removed []string
	for _, sub := range previous.Subscriptions {
		kept := next != nil && slices.ContainsFunc(next.Subscriptions, func(n appv1alpha1.DatabaseLogicalSubscription) bool {
			return n.PasswordSecret == sub.PasswordSecret
		})
		if !kept {
			removed = append(removed, sub.PasswordSecret)
		}
	}
	return removed
}

// deleteSubscriptionSecrets deletes the named password Secrets once no spec
// names them, and returns what it could not delete. CNPG's instance manager
// has already written each password to its passfile, which the subscription's
// drop uses.
func (s *Service) deleteSubscriptionSecrets(ctx context.Context, d *appv1alpha1.Database, names []string) []string {
	var problems []string
	for _, name := range names {
		sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: d.Namespace}}
		if err := s.Client.Delete(ctx, sec); err != nil && !apierrors.IsNotFound(err) {
			problems = append(problems, "deleting the password Secret "+name+": "+err.Error())
		}
	}
	return problems
}
