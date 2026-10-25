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
package v1alpha1

import "time"

// The nightly disk snapshot contract the operator schedules and bex-api judges
// (w6/m152). Both sides read these, so "is a snapshot overdue?" cannot drift
// from when the operator actually runs one.
const (
	// DiskBackupHourUTC opens the nightly window; each disk's minute within the
	// hour is spread by a hash of its App name.
	DiskBackupHourUTC = 2
	// DiskBackupStartingDeadline is how late a missed start may still run.
	DiskBackupStartingDeadline = time.Hour
	// DiskBackupActiveDeadline bounds one snapshot run.
	DiskBackupActiveDeadline = 6 * time.Hour
)

// DiskReady reasons for a disk whose nightly snapshots cannot be scheduled.
const (
	DiskReasonSnapshotImageUnresolved       = "SnapshotImageUnresolved"
	DiskReasonSnapshotCredentialUnavailable = "SnapshotCredentialUnavailable"
)

// DiskRestoreRequestedAtAnnotation stamps each owner restore request (bex-api's
// RestoreDiskSnapshot). With spec.disk.restoreSnapshot it identifies one
// request, so the same snapshot can be restored again and a failed restore can
// be retried by asking again. A CR without it keeps the snapshot alone as the
// request (w6/089).
const DiskRestoreRequestedAtAnnotation = "app.bex.co/disk-restore-requested-at"
