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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A standby's stale logical slots (w34/008; docs/ADR009 § Logical
// replication › Lifecycle › Failover and switchover).
//
// On an HA publisher running PostgreSQL 17 or later, the standby's slot
// synchronization copies each slot created with failover = true and keeps it
// current, marking it synced. It never replaces a slot it did not create. An
// old primary that rejoins as the standby without pg_rewind, after a clean
// shutdown (a switchover, or a Pod deletion that waited for its sessions),
// keeps every logical slot it had as the primary, synced = false, at the
// position it stopped. The sync worker then exits on the first of them
// ("exiting from slot synchronization because same name slot ... already
// exists on the standby", checked on 17.11 and 18.7) and synchronizes no slot
// at all, and the next failover onto that instance resumes each subscription
// from the stale copy, which holds back WAL until max_slot_wal_keep_size
// invalidates it. CloudNativePG 1.30.2 does not clean them up
// (cloudnative-pg/cloudnative-pg#9969, open, its fixes unmerged).
//
// So bex-api drops them. No URL reaches the HA standby (read replicas are
// Clusters of their own, ADR009 § Named read replicas), so a logical slot
// there that the sync worker did not create was left from the instance's time
// as the primary. Its WAL and its catalog_xmin, which hot_standby_feedback
// sends to the primary, serve nobody. Dropping one takes the REPLICATION
// attribute, which CloudNativePG's streaming_replica has, and works from any
// database of the instance, so the sweep connects as streaming_replica to the
// postgres database, the one CloudNativePG's pg_hba lets it reach by client
// certificate.

// standbySlotSweepInterval is how often the standbys are checked: the slot
// sync worker's longest nap, so a rejoined standby synchronizes again within
// about two of them.
const standbySlotSweepInterval = 30 * time.Second

// CloudNativePG's names: the label that marks an instance's role, the
// replication role, and the Secret holding its client certificate.
const (
	cnpgInstanceRoleLabel     = "cnpg.io/instanceRole"
	cnpgReplicaInstanceRole   = "replica"
	cnpgStreamingReplicaUser  = "streaming_replica"
	cnpgReplicationCertSuffix = "-replication"
)

// databaseServerCASuffixes name the Secrets whose ca.crt may anchor the
// instances' server certificate: CloudNativePG's own CA, and the roots of a
// public database's issued certificate, which the operator writes and the
// Cluster serves once it is issued (database_public_tls.go in the operator).
var databaseServerCASuffixes = []string{"-ca", "-public-ca"}

// StandbySlotSweeper drops the logical slots an HA publisher's standby kept
// from its time as the primary, so its slot synchronization runs again.
// Concurrent bex-api replicas may both sweep: a slot already dropped is
// skipped.
type StandbySlotSweeper struct {
	Service *Service
	// Interval between sweeps; non-positive means standbySlotSweepInterval.
	Interval time.Duration
	// sweepStandby is the per-instance step; nil connects and drops
	// (dropStaleStandbySlots). Tests replace it.
	sweepStandby func(ctx context.Context, cfg *pgx.ConnConfig) ([]string, error)
}

// Run sweeps until ctx is canceled. A nil sweeper or Service is a no-op.
func (w *StandbySlotSweeper) Run(ctx context.Context) {
	if w == nil || w.Service == nil || w.Service.Base == nil || w.Service.Client == nil {
		return
	}
	interval := w.Interval
	if interval <= 0 {
		interval = standbySlotSweepInterval
	}
	core.Poll(ctx, "postgres standby logical slots", interval, w.SweepOnce)
}

// SweepOnce checks the standby of every HA publisher.
func (w *StandbySlotSweeper) SweepOnce(ctx context.Context) error {
	var list appv1alpha1.DatabaseList
	if err := w.Service.Client.List(ctx, &list); err != nil {
		return fmt.Errorf("list databases: %w", err)
	}
	var errs []error
	for i := range list.Items {
		if ctx.Err() != nil {
			break
		}
		d := &list.Items[i]
		if !standbySynchronizesSlots(d) {
			continue
		}
		if err := w.sweepDatabase(ctx, d); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", d.Name, err))
		}
	}
	return errors.Join(errs...)
}

// standbySynchronizesSlots reports whether d's standby may synchronize
// logical slots: the operator's rule (syncsLogicalSlots), less the major,
// which each standby reports itself. A suspended or deleted database has no
// instance to reach.
func standbySynchronizesSlots(d *appv1alpha1.Database) bool {
	return d.DeletionTimestamp.IsZero() && !d.Spec.Suspended && d.Spec.HighAvailability &&
		d.Spec.LogicalReplication.Publishes()
}

// sweepDatabase drops the stale slots on each running standby of d's own
// Cluster.
func (w *StandbySlotSweeper) sweepDatabase(ctx context.Context, d *appv1alpha1.Database) error {
	var pods corev1.PodList
	if err := w.Service.Client.List(ctx, &pods, client.InNamespace(d.Namespace), client.MatchingLabels{
		core.PodLabelCNPGCluster: d.Name, cnpgInstanceRoleLabel: cnpgReplicaInstanceRole,
	}); err != nil {
		return fmt.Errorf("list standbys: %w", err)
	}
	var standbys []corev1.Pod
	for _, pod := range pods.Items {
		if pod.DeletionTimestamp.IsZero() && pod.Status.Phase == corev1.PodRunning && pod.Status.PodIP != "" {
			standbys = append(standbys, pod)
		}
	}
	if len(standbys) == 0 {
		return nil
	}
	tlsConfig, err := w.Service.streamingReplicaTLS(ctx, d)
	if err != nil {
		return err
	}
	sweep := w.sweepStandby
	if sweep == nil {
		sweep = dropStaleStandbySlots
	}
	var errs []error
	for _, pod := range standbys {
		cfg, err := standbyConnConfig(pod.Status.PodIP, tlsConfig)
		if err != nil {
			return err
		}
		dropped, err := sweep(ctx, cfg)
		for _, slot := range dropped {
			log.Printf("postgres standby logical slots: %s: dropped %q on standby %s, kept from its time as the primary, so slot synchronization can recreate it", d.Name, slot, pod.Name)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("standby %s: %w", pod.Name, err))
		}
	}
	return errors.Join(errs...)
}

// streamingReplicaTLS is what a connection as streaming_replica presents and
// trusts: CloudNativePG's replication client certificate, and the database's
// server CAs, checked as CloudNativePG's own replication checks them
// (verify-ca): an instance's Pod IP is on no certificate.
func (s *Service) streamingReplicaTLS(ctx context.Context, d *appv1alpha1.Database) (*tls.Config, error) {
	var replication corev1.Secret
	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: d.Namespace, Name: d.Name + cnpgReplicationCertSuffix}, &replication); err != nil {
		return nil, fmt.Errorf("read the replication client certificate: %w", err)
	}
	cert, err := tls.X509KeyPair(replication.Data[corev1.TLSCertKey], replication.Data[corev1.TLSPrivateKeyKey])
	if err != nil {
		return nil, fmt.Errorf("the replication client certificate is malformed: %w", err)
	}
	roots := x509.NewCertPool()
	anchored := false
	for _, suffix := range databaseServerCASuffixes {
		var ca corev1.Secret
		err := s.Client.Get(ctx, client.ObjectKey{Namespace: d.Namespace, Name: d.Name + suffix}, &ca)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read the server CA %s: %w", d.Name+suffix, err)
		}
		anchored = roots.AppendCertsFromPEM(ca.Data["ca.crt"]) || anchored
	}
	if !anchored {
		return nil, errors.New("no server CA to verify the instances with")
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		// The chain is checked in VerifyPeerCertificate, the host name not.
		InsecureSkipVerify:    true,
		VerifyPeerCertificate: verifyServerChain(roots),
	}, nil
}

// verifyServerChain checks the server's chain against roots, ignoring its
// names: libpq's sslmode=verify-ca.
func verifyServerChain(roots *x509.CertPool) func([][]byte, [][]*x509.Certificate) error {
	return func(raw [][]byte, _ [][]*x509.Certificate) error {
		if len(raw) == 0 {
			return errors.New("the server sent no certificate")
		}
		certs := make([]*x509.Certificate, len(raw))
		for i, der := range raw {
			cert, err := x509.ParseCertificate(der)
			if err != nil {
				return fmt.Errorf("parse the server certificate: %w", err)
			}
			certs[i] = cert
		}
		intermediates := x509.NewCertPool()
		for _, cert := range certs[1:] {
			intermediates.AddCert(cert)
		}
		_, err := certs[0].Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates})
		return err
	}
}

// standbyConnConfig dials one instance as streaming_replica into the
// postgres database, with the console's session rails.
func standbyConnConfig(podIP string, tlsConfig *tls.Config) (*pgx.ConnConfig, error) {
	cfg, err := buildQueryConnConfig(fmt.Sprintf("host=%s port=5432 dbname=postgres user=%s sslmode=require application_name=bex-standby-slots",
		podIP, cnpgStreamingReplicaUser), logicalReplicationLimits, false)
	if err != nil {
		return nil, err
	}
	cfg.ConnectTimeout = logicalReplicationConnectTimeout
	cfg.TLSConfig, cfg.Fallbacks = tlsConfig, nil
	return cfg, nil
}

// dropStaleStandbySlots connects with cfg and drops the instance's stale
// logical slots, returning their names.
func dropStaleStandbySlots(ctx context.Context, cfg *pgx.ConnConfig) ([]string, error) {
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close(context.Background()) }()
	return dropStaleSlots(ctx, conn)
}

// staleSlotFilter is a logical slot the sync worker did not create (synced
// is false), that no walsender holds, and that outlives its session.
const staleSlotFilter = `slot_type = 'logical' AND NOT synced AND NOT active AND NOT temporary`

// dropStaleSlots drops the stale logical slots of a standby on PostgreSQL 17
// or later, and does nothing on a primary, whose slots are the subscribers'
// own, or before 17, which has no slot synchronization (and no synced
// column). Each drop checks again, in the same statement, that the instance
// is still in recovery and the slot still stale: an instance promoted since
// keeps its slots, and one taken by a walsender is left.
func dropStaleSlots(ctx context.Context, conn *pgx.Conn) ([]string, error) {
	var version int
	var inRecovery bool
	if err := conn.QueryRow(ctx, `SELECT current_setting('server_version_num')::int, pg_catalog.pg_is_in_recovery()`).Scan(&version, &inRecovery); err != nil {
		return nil, mapPGError(err)
	}
	if version/10000 < appv1alpha1.LogicalSlotsCarriedMajor || !inRecovery {
		return nil, nil
	}
	rows, err := conn.Query(ctx, `SELECT slot_name::text FROM pg_catalog.pg_replication_slots WHERE `+staleSlotFilter+` ORDER BY slot_name`)
	if err != nil {
		return nil, mapPGError(err)
	}
	stale, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, mapPGError(err)
	}
	var dropped []string
	for _, slot := range stale {
		tag, err := conn.Exec(ctx, `SELECT pg_catalog.pg_drop_replication_slot(slot_name) FROM pg_catalog.pg_replication_slots
			WHERE slot_name = $1 AND `+staleSlotFilter+` AND pg_catalog.pg_is_in_recovery()`, slot)
		var pgErr *pgconn.PgError
		switch {
		case errors.As(err, &pgErr) && (pgErr.Code == pgerrcode.ObjectInUse || pgErr.Code == pgerrcode.UndefinedObject):
			// Taken by a walsender, or dropped by another sweep, since the list.
		case err != nil:
			return dropped, mapPGError(err)
		case tag.RowsAffected() == 1:
			dropped = append(dropped, slot)
		}
	}
	return dropped, nil
}
