-- w6/m566 t001: record an agent session's sandbox's used CPU and allocated
-- memory too, shown at $0 beside the weighted sandbox_compute_seconds charge
-- that still bills it.
--
-- Until now a sandbox CPU or memory row was unbilled only by its hour: before
-- sandboxUsage.billedFrom (pricing.yaml) every sandbox was billed on the
-- weighted meter and the two were shown beside it. An agent session's sandbox
-- keeps the weighted meter after that instant, so its CPU and memory rows are
-- unbilled whatever their hour. The meter knows which sandbox it is writing
-- for and marks the row here; every read that prices, exports, settles or
-- compacts usage leaves a marked row out of the bill, exactly as it leaves out
-- a row from before the switch. A sandbox's row of one hour is either marked or
-- not: one sandbox is billed one way in a given hour.
ALTER TABLE usage_hourly
    ADD COLUMN unbilled boolean NOT NULL DEFAULT false;
