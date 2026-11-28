-- +goose Up
-- The gpose staff upload for a completed sign-up, for the visitor to download from
-- their festival dashboard. It is kept a day: photo_token names its file in the data
-- folder's gpose-photos/ (and its address), photo_type is its content type, and
-- photo_expires_at is when the sweep deletes the file and clears all three. A read
-- after that moment answers as if there were none. '' and NULL mean no photo.
ALTER TABLE gpose_signups ADD COLUMN photo_token TEXT NOT NULL DEFAULT '';
ALTER TABLE gpose_signups ADD COLUMN photo_type TEXT NOT NULL DEFAULT '';
ALTER TABLE gpose_signups ADD COLUMN photo_expires_at DATETIME CHECK (photo_expires_at IS strftime('%Y-%m-%dT%H:%M:%SZ', photo_expires_at));
CREATE UNIQUE INDEX idx_gpose_signups_photo ON gpose_signups(photo_token) WHERE photo_token != '';
-- The sweep's due read.
CREATE INDEX idx_gpose_signups_photo_expiry ON gpose_signups(photo_expires_at) WHERE photo_expires_at IS NOT NULL;
PRAGMA user_version = 80;
