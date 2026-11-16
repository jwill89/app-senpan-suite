-- +goose Up
-- The sites whose pages may show a festival map in a frame, a JSON array of origins.
-- It starts as the sites that may read the public tea-room API, the Carrd pages that
-- show both; saved empty, no site may frame the map.
INSERT OR IGNORE INTO settings (key, value)
SELECT 'embed_origins', value FROM settings WHERE key = 'tea_room_origins';
PRAGMA user_version = 77;
