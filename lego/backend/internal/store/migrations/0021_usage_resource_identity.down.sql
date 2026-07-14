ALTER TABLE usage_monthly DROP CONSTRAINT usage_monthly_pkey;
ALTER TABLE usage_monthly
    ADD PRIMARY KEY (service_id, kind, tier, month);

ALTER TABLE usage_hourly DROP CONSTRAINT usage_hourly_pkey;
ALTER TABLE usage_hourly
    ADD PRIMARY KEY (service_id, kind, tier, window_start);
