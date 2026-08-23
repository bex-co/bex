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

// Command disk-snapshot is the backup and restore stage of persistent service
// disks (docs/ADR082-persistent-disks.md D5), as a first-party entrypoint of
// the bex image.
//
// Hetzner offers no volume snapshots — server snapshots exclude attached
// volumes and the CSI driver has no snapshot capability — so Render's disk
// snapshots are reproduced at the file level. One process reads the mounted
// volume and streams tar → gzip → age straight into a multipart upload, so the
// tenant's PLAINTEXT never lands on a staging disk and never crosses a
// container boundary. That is the same reason /backup-encrypt exists rather
// than an alpine container fetching the age release at run time (ADR068 #9):
// anything resolved at run time would execute next to unencrypted tenant data.
//
// Usage:
//
//	disk-snapshot backup    # /disk -> <prefix>/<ts>.tar.gz.age, then prune to retain
//	disk-snapshot restore   # <key> -> /disk, replacing its contents
//	disk-snapshot purge     # delete every snapshot for this disk
//
// Configuration is by environment (the Job's env + the store credential
// Secret); no flags, so a Job template is the only thing that decides what a
// run touches.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/disksnapshot"
)

const (
	// defaultMountPath is where the Job mounts the tenant's volume. It is the
	// Job's own path, not the App's spec.disk.mountPath: what is snapshotted is
	// the volume's contents, wherever the service happens to mount it.
	defaultMountPath = "/disk"
	// defaultRetain matches Render's documented "available for at least seven
	// days" and the KeyValue backup retention beside it.
	defaultRetain = 7
)

func main() {
	if len(os.Args) < 2 {
		fatal(fmt.Errorf("usage: disk-snapshot backup|restore|purge"))
	}
	// A snapshot of a large volume is a long upload; the deadline belongs to
	// the Job (activeDeadlineSeconds), so this context is cancellation-only.
	ctx := context.Background()
	var err error
	switch os.Args[1] {
	case "backup":
		err = backup(ctx)
	case "restore":
		err = restore(ctx)
	case "purge":
		err = purge(ctx)
	default:
		err = fmt.Errorf("unknown command %q (want backup, restore or purge)", os.Args[1])
	}
	if err != nil {
		fatal(err)
	}
}

func backup(ctx context.Context) error {
	store, prefix, err := storeFromEnv()
	if err != nil {
		return err
	}
	recipient := os.Getenv("AGE_PUBLIC_KEY")
	if recipient == "" {
		// Fail closed. An unencrypted snapshot in a third-party bucket is worse
		// than no snapshot, and it would be indistinguishable from a good one.
		return fmt.Errorf("AGE_PUBLIC_KEY is required; refusing to write an unencrypted snapshot")
	}
	mount := envOr("BEX_DISK_MOUNT_PATH", defaultMountPath)
	if err := requireDirectory(mount); err != nil {
		return err
	}

	key := disksnapshot.SnapshotKey(prefix, time.Now())
	// The pipe is what makes this one pass: Backup writes the encrypted stream
	// as it walks the tree, and the uploader reads it as parts. Neither side
	// ever holds the whole volume.
	reader, writer := io.Pipe()
	go func() {
		writer.CloseWithError(disksnapshot.Backup(mount, writer, recipient))
	}()
	if err := store.Put(ctx, key, reader); err != nil {
		return err
	}
	log.Printf("disk-snapshot: wrote %s", key)

	retain := envInt("BEX_DISK_SNAPSHOT_RETAIN", defaultRetain)
	deleted, err := store.Prune(ctx, prefix, retain)
	if err != nil {
		// The snapshot is already safe; a failed prune costs storage, not data.
		return fmt.Errorf("snapshot written but retention sweep failed: %w", err)
	}
	if len(deleted) > 0 {
		log.Printf("disk-snapshot: pruned %d snapshot(s) beyond the newest %d", len(deleted), retain)
	}
	return nil
}

func restore(ctx context.Context) error {
	store, prefix, err := storeFromEnv()
	if err != nil {
		return err
	}
	identity := os.Getenv("AGE_PRIVATE_KEY")
	if identity == "" {
		return fmt.Errorf("AGE_PRIVATE_KEY is required to decrypt a snapshot")
	}
	key := os.Getenv("BEX_DISK_SNAPSHOT_KEY")
	if key == "" {
		return fmt.Errorf("BEX_DISK_SNAPSHOT_KEY is required (which snapshot to restore)")
	}
	// Confine the restore to this disk's own prefix. The key reaches here from
	// an API request, so a key naming another tenant's object must not be
	// fetchable even if the signature that carried it were ever mis-issued.
	if len(key) < len(prefix) || key[:len(prefix)] != prefix {
		return fmt.Errorf("snapshot %q does not belong to this disk", key)
	}
	mount := envOr("BEX_DISK_MOUNT_PATH", defaultMountPath)
	if err := requireDirectory(mount); err != nil {
		return err
	}

	body, err := store.Get(ctx, key)
	if err != nil {
		return err
	}
	defer func() { _ = body.Close() }()
	// Restore clears the volume before extracting, which is Render's behavior:
	// everything written after the snapshot is discarded. It is safe to retry —
	// the snapshot stays in the store — but it is not undoable.
	if err := disksnapshot.Restore(mount, body, identity); err != nil {
		return fmt.Errorf("restore %s: %w", key, err)
	}
	log.Printf("disk-snapshot: restored %s", key)
	return nil
}

func purge(ctx context.Context) error {
	store, prefix, err := storeFromEnv()
	if err != nil {
		return err
	}
	n, err := store.PurgeAll(ctx, prefix)
	if err != nil {
		return err
	}
	log.Printf("disk-snapshot: purged %d snapshot(s)", n)
	return nil
}

// storeFromEnv builds the store and this disk's prefix. Every field is
// required: a half-configured store would silently write somewhere unintended.
func storeFromEnv() (disksnapshot.Store, string, error) {
	store := disksnapshot.Store{
		Endpoint: os.Getenv("BEX_DISK_SNAPSHOT_ENDPOINT"),
		Bucket:   os.Getenv("BEX_DISK_SNAPSHOT_BUCKET"),
		Prefix:   os.Getenv("BEX_DISK_SNAPSHOT_PREFIX"),
		Region:   os.Getenv("BEX_DISK_SNAPSHOT_REGION"),
	}
	workspace, disk := os.Getenv("BEX_DISK_WORKSPACE"), os.Getenv("BEX_DISK_ID")
	switch {
	case store.Endpoint == "":
		return store, "", fmt.Errorf("BEX_DISK_SNAPSHOT_ENDPOINT is required")
	case store.Bucket == "":
		return store, "", fmt.Errorf("BEX_DISK_SNAPSHOT_BUCKET is required")
	case workspace == "":
		return store, "", fmt.Errorf("BEX_DISK_WORKSPACE is required")
	case disk == "":
		return store, "", fmt.Errorf("BEX_DISK_ID is required")
	}
	return store, disksnapshot.DiskPrefix(workspace, disk), nil
}

// requireDirectory fails loudly when the volume is not mounted. Without it a
// backup of an unmounted path would upload an empty archive that looks like a
// perfectly good snapshot, and a restore would extract into the pod's own
// filesystem instead of the volume.
func requireDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("disk mount %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("disk mount %s is not a directory", path)
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil || v < 1 {
		return fallback
	}
	return v
}

func fatal(err error) {
	log.Printf("disk-snapshot: %v", err)
	os.Exit(1)
}
