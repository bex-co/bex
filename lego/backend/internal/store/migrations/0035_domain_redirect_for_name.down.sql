ALTER TABLE domains
DROP CONSTRAINT domains_redirect_not_self,
DROP COLUMN redirect_for_name;
