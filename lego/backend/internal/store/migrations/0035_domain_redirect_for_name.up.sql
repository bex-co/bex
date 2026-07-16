ALTER TABLE domains
ADD COLUMN redirect_for_name text,
ADD CONSTRAINT domains_redirect_not_self CHECK (
    redirect_for_name IS NULL OR redirect_for_name <> host
);
