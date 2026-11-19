DROP TRIGGER IF EXISTS projects_default_saved_searches ON projects;
DROP FUNCTION IF EXISTS bex_default_saved_searches();
DROP TABLE IF EXISTS saved_searches;
DROP FUNCTION IF EXISTS bex_saved_search_id(text, text);
