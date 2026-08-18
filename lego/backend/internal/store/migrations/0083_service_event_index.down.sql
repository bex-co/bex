DROP TRIGGER IF EXISTS service_event_facts_event_index_delete ON service_event_facts;
DROP TRIGGER IF EXISTS audit_events_service_event_index_delete ON audit_events;
DROP TRIGGER IF EXISTS deploys_service_event_index_delete ON deploys;
DROP TRIGGER IF EXISTS service_event_facts_event_index_insert ON service_event_facts;
DROP TRIGGER IF EXISTS audit_events_service_event_index_insert ON audit_events;
DROP TRIGGER IF EXISTS deploys_service_event_index_finish ON deploys;
DROP TRIGGER IF EXISTS deploys_service_event_index_insert ON deploys;

DROP FUNCTION IF EXISTS bex_delete_service_event_index();
DROP FUNCTION IF EXISTS bex_index_fact_service_event();
DROP FUNCTION IF EXISTS bex_index_audit_service_event();
DROP FUNCTION IF EXISTS bex_index_deploy_service_events();

DROP TABLE IF EXISTS service_event_index;
DROP FUNCTION IF EXISTS bex_service_event_id(text);
