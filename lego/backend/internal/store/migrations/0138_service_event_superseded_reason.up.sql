-- The reconciler already emits superseded for canceled build-ended facts.
-- Admit that typed reason without changing ordinary user cancellation ('').
ALTER TABLE service_event_facts DROP CONSTRAINT service_event_facts_reason_code_check;
ALTER TABLE service_event_facts ADD CONSTRAINT service_event_facts_reason_code_check
    CHECK (reason_code IN (
        '',
        'image_pull_backoff',
        'readiness_failed',
        'root_directory',
        'build_filter',
        'skip_phrase',
        'superseded'
    ));
