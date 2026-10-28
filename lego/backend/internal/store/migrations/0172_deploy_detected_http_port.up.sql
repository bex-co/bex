ALTER TABLE deploys ADD COLUMN detected_http_port integer NOT NULL DEFAULT 0
    CHECK (detected_http_port BETWEEN 0 AND 65535);
