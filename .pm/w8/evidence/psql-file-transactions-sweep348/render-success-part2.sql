INSERT INTO qa348_1025fa_render_success VALUES(3,E'line one\n日本語 café 🐟'),(4,''),(5,NULL);
SELECT jsonb_agg(jsonb_build_object('id',id,'v',v) ORDER BY id)::text FROM qa348_1025fa_render_success;
