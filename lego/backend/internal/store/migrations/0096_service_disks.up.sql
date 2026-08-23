-- w1/m84: persistent service disks (docs/ADR082-persistent-disks.md).
--
-- A disk is workspace-owned metadata attached to exactly one service, and the
-- control plane is its source of truth: the operator derives spec.disk from
-- these rows, and the billing meter derives provisioned GB-seconds from them
-- too. Both cascades are deliberate — deleting a service or a workspace must
-- not leave a row that keeps billing for a volume nobody can reach.
CREATE TABLE service_disks (
    id         text PRIMARY KEY,
    tenant_id  text NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    app_id     text NOT NULL REFERENCES apps (id) ON DELETE CASCADE,
    name       text NOT NULL,
    mount_path text NOT NULL,
    size_gb    integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    -- The CRD's own bounds (ADR082 D2), repeated here so a bad row cannot be
    -- written by any path that skips the API's validation.
    CONSTRAINT service_disks_size_gb_check CHECK (size_gb BETWEEN 1 AND 10000),
    CONSTRAINT service_disks_name_check CHECK (char_length(name) BETWEEN 1 AND 63),
    CONSTRAINT service_disks_mount_path_check
        CHECK (mount_path ~ '^/.*' AND char_length(mount_path) BETWEEN 1 AND 255)
);

-- Render allows at most one disk per service, and the App CR can only express
-- one. Enforcing it here means a racing double-create loses on the database
-- rather than silently producing a second row the projector would ignore.
CREATE UNIQUE INDEX service_disks_one_live_per_app_idx
    ON service_disks (app_id)
    WHERE deleted_at IS NULL;

CREATE INDEX service_disks_tenant_live_idx
    ON service_disks (tenant_id, created_at)
    WHERE deleted_at IS NULL;

-- The size the disk held over a period of time. Disks are billed on PROVISIONED
-- capacity (ADR082 D8), so the meter integrates size over time rather than
-- sampling: a grow that lands mid-hour has to contribute the old size up to the
-- change and the new size after it. The current size is denormalized onto
-- service_disks for reads; this table is what makes the invoice reproducible.
--
-- to_ts NULL means "still at this size". A disk's deletion closes its open
-- period, which is also what stops the billing.
CREATE TABLE service_disk_sizes (
    disk_id text NOT NULL REFERENCES service_disks (id) ON DELETE CASCADE,
    size_gb integer NOT NULL,
    from_ts timestamptz NOT NULL,
    to_ts   timestamptz,
    PRIMARY KEY (disk_id, from_ts),
    CONSTRAINT service_disk_sizes_size_gb_check CHECK (size_gb BETWEEN 1 AND 10000),
    CONSTRAINT service_disk_sizes_order_check CHECK (to_ts IS NULL OR to_ts >= from_ts)
);

-- The meter walks periods overlapping an hour window, per workspace.
CREATE INDEX service_disk_sizes_window_idx
    ON service_disk_sizes (disk_id, from_ts, to_ts);

-- The provisioned-disk meter (ADR082 D9). It is a distinct kind from
-- storage_gb_seconds on purpose: that one samples USED bytes on a datastore
-- PVC and prices at $0.21/GB-month, while this one integrates PROVISIONED
-- capacity and prices at $0.175 — same unit, different basis and different
-- rate, so folding them would make one invoice line unreproducible.
ALTER TABLE usage_hourly DROP CONSTRAINT usage_hourly_kind_check;
ALTER TABLE usage_hourly ADD CONSTRAINT usage_hourly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds'));

ALTER TABLE usage_monthly DROP CONSTRAINT usage_monthly_kind_check;
ALTER TABLE usage_monthly ADD CONSTRAINT usage_monthly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds'));
