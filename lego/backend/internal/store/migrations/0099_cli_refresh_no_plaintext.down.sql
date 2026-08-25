-- Down is structural only: the scrubbed response bytes cannot be recovered
-- (that is the point). Old binaries reading an emptied row see an empty body.
SELECT 1;
