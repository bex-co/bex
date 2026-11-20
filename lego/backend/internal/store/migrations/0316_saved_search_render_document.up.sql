-- 0316: a view's filter document in Render's spellings and shape (w3/083,
-- ADR010 § Project views). Render's dashboard writes Text Search as
-- TEXT_SEARCH with CONTAINS and keeps the time range out of `filters`, as the
-- top-level timeRange.relative; bex wrote TEXT with IS and a TIME_RANGE row.
-- Every stored view moves to Render's shape. bex-api still reads the old
-- spellings (internal/savedsearch), so a document an older replica writes
-- while this rolls out still reads, and is served in Render's shape.
UPDATE saved_searches s
SET filters = (s.filters - 'filters')
    || jsonb_build_object('filters', COALESCE((
        SELECT jsonb_agg(
            CASE WHEN f ->> 'field' = 'TEXT'
                THEN f || jsonb_build_object(
                    'field', 'TEXT_SEARCH',
                    'operator', CASE WHEN f ->> 'operator' = 'IS' THEN to_jsonb('CONTAINS'::text) ELSE f -> 'operator' END)
                ELSE f
            END ORDER BY e.ord)
        FROM jsonb_array_elements(s.filters -> 'filters') WITH ORDINALITY AS e (f, ord)
        WHERE f ->> 'field' IS DISTINCT FROM 'TIME_RANGE'
    ), '[]'::jsonb))
    || COALESCE((
        SELECT jsonb_build_object('timeRange', jsonb_build_object('relative', f -> 'values' -> 0))
        FROM jsonb_array_elements(s.filters -> 'filters') AS e (f)
        WHERE f ->> 'field' = 'TIME_RANGE'
        LIMIT 1
    ), '{}'::jsonb)
-- Neither test can raise, so their order does not matter: only a row whose
-- filters are an array holding an old spelling reaches the SET.
WHERE jsonb_typeof(s.filters -> 'filters') = 'array'
  AND jsonb_path_exists(s.filters, '$.filters[*] ? (@.field == "TEXT" || @.field == "TIME_RANGE")');
