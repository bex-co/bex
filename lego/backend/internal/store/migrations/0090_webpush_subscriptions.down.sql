ALTER TABLE push_deliveries
    ADD CONSTRAINT push_deliveries_tenant_id_subject_device_id_fkey
    FOREIGN KEY (tenant_id, subject, device_id)
    REFERENCES device_push_subscriptions (tenant_id, subject, device_id)
    ON DELETE CASCADE;

DROP TABLE webpush_subscriptions;
