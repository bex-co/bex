-- 0316 down: back to bex's earlier document, the only one the bex-api before
-- w3/083 reads: TEXT_SEARCH is TEXT with IS again, timeRange.relative a
-- TIME_RANGE row after the others, and the document holds `filters` and
-- `version` alone. That bex-api refuses any other key, so a view's `display`
-- (Render's metrics display options, which bex never read) is dropped.
UPDATE saved_searches s
SET filters = jsonb_build_object(
    'version', COALESCE(s.filters -> 'version', '1'::jsonb),
    'filters', COALESCE((
        SELECT jsonb_agg(
            CASE WHEN f ->> 'field' = 'TEXT_SEARCH'
                THEN f || '{"field": "TEXT", "operator": "IS"}'::jsonb
                ELSE f
            END ORDER BY e.ord)
        FROM jsonb_array_elements(
            CASE WHEN jsonb_typeof(s.filters -> 'filters') = 'array' THEN s.filters -> 'filters' ELSE '[]'::jsonb END
        ) WITH ORDINALITY AS e (f, ord)
    ), '[]'::jsonb)
    || CASE WHEN jsonb_typeof(s.filters #> '{timeRange,relative}') = 'string'
        THEN jsonb_build_array(jsonb_build_object(
            'field', 'TIME_RANGE', 'operator', 'IS', 'values', jsonb_build_array(s.filters #> '{timeRange,relative}')))
        ELSE '[]'::jsonb
    END)
WHERE s.filters ? 'timeRange'
   OR s.filters ? 'display'
   OR jsonb_path_exists(s.filters, '$.filters[*] ? (@.field == "TEXT_SEARCH")');
