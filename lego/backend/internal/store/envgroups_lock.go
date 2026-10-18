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

package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type envGroupSessionKey struct{}

type envGroupSession struct {
	store *PGStore
	conn  *pgxpool.Conn
	held  map[string]bool
}

type envGroupQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func (s *PGStore) envGroupQuery(ctx context.Context) envGroupQuerier {
	if session, ok := ctx.Value(envGroupSessionKey{}).(*envGroupSession); ok && session.store == s {
		return session.conn
	}
	return s.Pool
}

// WithEnvGroupContentLock serializes a group's external value/projection effects
// across API replicas. A lease permits recovery only after the previous live
// writer releases its session (or crashes); it never licenses overwriting a
// stalled writer's later effects. The callback's group queries use this same
// connection, including detached compensation, so a one-connection pool works.
// Statements commit individually: a crash must retain the busy claim and op.
// The callback must not use the supplied context concurrently or retain it.
func (s *PGStore) WithEnvGroupContentLock(ctx context.Context, id string, fn func(context.Context) error) (err error) {
	session, nested := ctx.Value(envGroupSessionKey{}).(*envGroupSession)
	if !nested || session.store != s {
		conn, acquireErr := s.Pool.Acquire(ctx)
		if acquireErr != nil {
			return acquireErr
		}
		defer conn.Release()
		session = &envGroupSession{store: s, conn: conn, held: map[string]bool{}}
		ctx = context.WithValue(ctx, envGroupSessionKey{}, session)
	}
	if session.held[id] {
		return fn(ctx)
	}
	var locked bool
	if err := session.conn.QueryRow(ctx,
		`SELECT pg_try_advisory_lock(hashtextextended($1, 0))`, "env-group-content:"+id).Scan(&locked); err != nil {
		// The server might have acquired it before the connection failed. Never
		// return a possibly locked session to the pool.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = session.conn.Conn().Close(cleanupCtx)
		return err
	}
	if !locked {
		return ErrEnvGroupChanged
	}
	session.held[id] = true
	defer func() {
		delete(session.held, id)
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var unlocked bool
		unlockErr := session.conn.QueryRow(cleanupCtx,
			`SELECT pg_advisory_unlock(hashtextextended($1, 0))`, "env-group-content:"+id).Scan(&unlocked)
		if unlockErr == nil && !unlocked {
			unlockErr = errors.New("environment group content lock was lost")
		}
		if unlockErr != nil {
			_ = session.conn.Conn().Close(cleanupCtx)
			err = errors.Join(err, unlockErr)
		}
	}()
	return fn(ctx)
}
