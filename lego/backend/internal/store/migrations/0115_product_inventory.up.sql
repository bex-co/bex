-- Complete point-in-time inventory, separate from daily "ever hosted" evidence.
ALTER TABLE product_analytics_collection ADD COLUMN inventory_started_at timestamptz NOT NULL DEFAULT clock_timestamp();
ALTER TABLE product_analytics_collection ADD COLUMN events_retained_from timestamptz;
CREATE TABLE product_inventory_batches (
    source text NOT NULL CHECK (source IN ('services','postgres','keyvalue','domains')),
    bucket timestamptz NOT NULL,
    observed_at timestamptz NOT NULL,
    complete boolean NOT NULL,
    PRIMARY KEY (source,bucket)
);
CREATE TABLE product_inventory_counts (
    source text NOT NULL,
    bucket timestamptz NOT NULL,
    workspace_id text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    resource_type text NOT NULL CHECK (resource_type IN ('web_service','static_site','private_service','background_worker','cron_job','postgres','keyvalue')),
    state text NOT NULL CHECK (state IN ('ready','provisioning','failed','suspended','unknown','canceled','deleting','unverified','verified_tls_ready','verified_tls_pending','verified_tls_unknown')),
    resources integer NOT NULL CHECK (resources > 0),
    PRIMARY KEY (source,bucket,workspace_id,resource_type,state),
    FOREIGN KEY (source,bucket) REFERENCES product_inventory_batches(source,bucket) ON DELETE CASCADE
);
CREATE INDEX product_inventory_batches_time_idx ON product_inventory_batches(bucket);
CREATE INDEX product_inventory_counts_workspace_idx ON product_inventory_counts(workspace_id);

-- One lifecycle checkpoint per resource, not one high-cardinality row per sample.
CREATE TABLE product_inventory_lifecycle (
    resource_id text PRIMARY KEY,
    workspace_id text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    source text NOT NULL CHECK (source IN ('services','postgres','keyvalue')),
    resource_type text NOT NULL CHECK (resource_type IN ('web_service','static_site','private_service','background_worker','cron_job','postgres','keyvalue')),
    created_at timestamptz NOT NULL,
    first_seen_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    first_ready_at timestamptz,
    removed_at timestamptz,
    state text NOT NULL CHECK (state IN ('ready','provisioning','failed','suspended','unknown','canceled','deleting'))
);
CREATE INDEX product_inventory_lifecycle_workspace_idx ON product_inventory_lifecycle(workspace_id);
CREATE INDEX product_inventory_lifecycle_source_idx ON product_inventory_lifecycle(source,last_seen_at);
CREATE INDEX product_inventory_lifecycle_created_idx ON product_inventory_lifecycle(created_at);
