-- Intentionally retain this additive constraint relaxation on rollback.
-- Older binaries already recognize and emit superseded. Narrowing the check
-- would reject their writes or require erasing attribution from retained facts.
-- No event rows or reason codes are rewritten or deleted.
SELECT 1;
