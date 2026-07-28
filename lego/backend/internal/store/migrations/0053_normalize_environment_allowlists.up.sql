-- w1/m56: retire the last bare-CIDR JSON compatibility decoder. Migration
-- 0034 deliberately preserved pre-existing text[] entries as JSON strings;
-- normalize those entries before the application starts decoding rows as the
-- required Render {cidrBlock, description} shape. Object entries pass through
-- unchanged and no description is fabricated for a legacy string.
UPDATE environments
SET ip_allow_list = (
    SELECT COALESCE(
        jsonb_agg(
            CASE jsonb_typeof(entry)
                WHEN 'string' THEN jsonb_build_object(
                    'cidrBlock', entry #>> '{}',
                    'description', ''
                )
                ELSE entry
            END
            ORDER BY ordinal
        ),
        '[]'::jsonb
    )
    FROM jsonb_array_elements(ip_allow_list) WITH ORDINALITY AS items(entry, ordinal)
)
WHERE EXISTS (
    SELECT 1
    FROM jsonb_array_elements(ip_allow_list) AS items(entry)
    WHERE jsonb_typeof(entry) = 'string'
);

ALTER TABLE environments
    ADD CONSTRAINT environments_ip_allow_list_structured CHECK (
        jsonb_typeof(ip_allow_list) = 'array'
        AND NOT jsonb_path_exists(
            ip_allow_list,
            '$[*] ? (@.type() != "object" || !exists(@.cidrBlock) || @.cidrBlock.type() != "string" || @.cidrBlock == "" || (exists(@.description) && @.description.type() != "string"))'
        )
    );
