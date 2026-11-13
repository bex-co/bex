-- ADR091 D10.2a (w6/m362 t016): a threshold charge collects only agent-model
-- usage no subscription invoice has billed. agent_model_billed_until is the
-- latest period_end of a bex subscription invoice that billed the meter (a
-- cycle, threshold, cancel or update invoice; never a standalone charge
-- invoice). It only moves forward. The charge window opens at the later of
-- the calendar month's start and this instant: hours that started before it
-- count as billed, and so do the paid charges made before it, which their
-- negative net items took out of that invoice. NULL: no such invoice seen yet.
ALTER TABLE billing_provider_mappings ADD COLUMN agent_model_billed_until timestamptz;
