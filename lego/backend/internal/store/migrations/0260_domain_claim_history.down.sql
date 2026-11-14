DROP TRIGGER IF EXISTS domains_clear_claim_history ON domains;
DROP TRIGGER IF EXISTS domains_inherit_claim_time ON domains;
DROP TRIGGER IF EXISTS domains_record_claim_history ON domains;
DROP FUNCTION IF EXISTS bex_clear_domain_claim_history();
DROP FUNCTION IF EXISTS bex_inherit_domain_claim_time();
DROP FUNCTION IF EXISTS bex_record_domain_claim_history();
DROP TABLE IF EXISTS domain_claim_history;
