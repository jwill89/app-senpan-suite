-- Gpose booths, the site's list of gpose types, and the sign-ups. A price is whole gil;
-- times are the stored UTC form. A sign-up holds its place or slot while it is waiting
-- or completed; a waitlisted one holds no slot.

-- name: ListGposeTypes :many
SELECT t.id, t.name, t.created_at,
       CAST((SELECT COUNT(*) FROM gpose_booth_types bt WHERE bt.type_id = t.id) AS INTEGER) AS booth_count
FROM gpose_types t ORDER BY t.id;

-- name: GetGposeType :one
SELECT id, name, created_at FROM gpose_types WHERE id = ?;

-- name: CreateGposeType :one
INSERT INTO gpose_types (name) VALUES (?) RETURNING id;

-- name: UpdateGposeType :execrows
UPDATE gpose_types SET name = ? WHERE id = ?;

-- name: CountGposeTypeBooths :one
SELECT COUNT(*) FROM gpose_booth_types WHERE type_id = ?;

-- name: DeleteGposeType :execrows
DELETE FROM gpose_types WHERE id = ?;

-- name: ListGposeBooths :many
SELECT b.id, b.title, b.description, b.mode, b.max_signups, b.max_waitlist, b.per_person, b.slot_step, b.status,
       b.festival_map_id, CAST(COALESCE(fm.title, '') AS TEXT) AS festival_map_name,
       CAST(COALESCE(fm.slug, '') AS TEXT) AS festival_map_slug, CAST(COALESCE(fm.status, '') AS TEXT) AS festival_map_status,
       b.occupant_id, CAST(COALESCE(fo.title, '') AS TEXT) AS stall_name, b.created_at,
       CAST((SELECT COUNT(*) FROM gpose_signups s
             WHERE s.booth_id = b.id AND s.waitlisted = 0 AND s.status IN ('waiting', 'completed')) AS INTEGER) AS booked_count,
       CAST((SELECT COUNT(*) FROM gpose_signups s
             WHERE s.booth_id = b.id AND s.waitlisted = 1 AND s.status = 'waiting') AS INTEGER) AS waitlist_count,
       CAST((SELECT COUNT(*) FROM gpose_signups s WHERE s.booth_id = b.id AND s.status = 'waiting') AS INTEGER) AS waiting_count
FROM gpose_booths b
LEFT JOIN festival_maps fm ON fm.id = b.festival_map_id
LEFT JOIN festival_stall_occupants fo ON fo.id = b.occupant_id
ORDER BY b.created_at DESC, b.id DESC;

-- name: GetGposeBooth :one
SELECT b.id, b.title, b.description, b.mode, b.max_signups, b.max_waitlist, b.per_person, b.slot_step, b.status,
       b.festival_map_id, CAST(COALESCE(fm.title, '') AS TEXT) AS festival_map_name,
       CAST(COALESCE(fm.slug, '') AS TEXT) AS festival_map_slug, CAST(COALESCE(fm.status, '') AS TEXT) AS festival_map_status,
       b.occupant_id, CAST(COALESCE(fo.title, '') AS TEXT) AS stall_name, b.created_at,
       CAST((SELECT COUNT(*) FROM gpose_signups s
             WHERE s.booth_id = b.id AND s.waitlisted = 0 AND s.status IN ('waiting', 'completed')) AS INTEGER) AS booked_count,
       CAST((SELECT COUNT(*) FROM gpose_signups s
             WHERE s.booth_id = b.id AND s.waitlisted = 1 AND s.status = 'waiting') AS INTEGER) AS waitlist_count,
       CAST((SELECT COUNT(*) FROM gpose_signups s WHERE s.booth_id = b.id AND s.status = 'waiting') AS INTEGER) AS waiting_count
FROM gpose_booths b
LEFT JOIN festival_maps fm ON fm.id = b.festival_map_id
LEFT JOIN festival_stall_occupants fo ON fo.id = b.occupant_id
WHERE b.id = ?;

-- name: ListFestivalGposeBooths :many
-- The open booths filed under a festival, oldest first.
SELECT b.id, b.title, b.description, b.mode, b.max_signups, b.max_waitlist, b.per_person, b.slot_step, b.status,
       b.festival_map_id, CAST(COALESCE(fm.title, '') AS TEXT) AS festival_map_name,
       CAST(COALESCE(fm.slug, '') AS TEXT) AS festival_map_slug, CAST(COALESCE(fm.status, '') AS TEXT) AS festival_map_status,
       b.occupant_id, CAST(COALESCE(fo.title, '') AS TEXT) AS stall_name, b.created_at,
       CAST((SELECT COUNT(*) FROM gpose_signups s
             WHERE s.booth_id = b.id AND s.waitlisted = 0 AND s.status IN ('waiting', 'completed')) AS INTEGER) AS booked_count,
       CAST((SELECT COUNT(*) FROM gpose_signups s
             WHERE s.booth_id = b.id AND s.waitlisted = 1 AND s.status = 'waiting') AS INTEGER) AS waitlist_count,
       CAST((SELECT COUNT(*) FROM gpose_signups s WHERE s.booth_id = b.id AND s.status = 'waiting') AS INTEGER) AS waiting_count
FROM gpose_booths b
LEFT JOIN festival_maps fm ON fm.id = b.festival_map_id
LEFT JOIN festival_stall_occupants fo ON fo.id = b.occupant_id
WHERE b.festival_map_id = ? AND b.status = 'open'
ORDER BY b.created_at ASC, b.id ASC;

-- name: CreateGposeBooth :one
INSERT INTO gpose_booths (title, description, mode, max_signups, max_waitlist, per_person, slot_step,
                          festival_map_id, occupant_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateGposeBooth :execrows
-- Everything a booth's form edits; its status is not.
UPDATE gpose_booths
SET title = ?, description = ?, mode = ?, max_signups = ?, max_waitlist = ?, per_person = ?, slot_step = ?,
    festival_map_id = ?, occupant_id = ?
WHERE id = ?;

-- name: SetGposeBoothStatus :execrows
UPDATE gpose_booths SET status = ? WHERE id = ?;

-- name: DeleteGposeBooth :execrows
-- Its types, open times and sign-ups go with it (cascade).
DELETE FROM gpose_booths WHERE id = ?;

-- name: ListGposeBoothTypes :many
SELECT bt.type_id, t.name, bt.price, bt.minutes
FROM gpose_booth_types bt JOIN gpose_types t ON t.id = bt.type_id
WHERE bt.booth_id = ? ORDER BY bt.sort_order, bt.id;

-- name: DeleteGposeBoothTypes :exec
DELETE FROM gpose_booth_types WHERE booth_id = ?;

-- name: InsertGposeBoothType :exec
INSERT INTO gpose_booth_types (booth_id, type_id, price, minutes, sort_order) VALUES (?, ?, ?, ?, ?);

-- name: ListGposeBoothWindows :many
SELECT start_at, end_at FROM gpose_booth_windows WHERE booth_id = ? ORDER BY start_at, id;

-- name: DeleteGposeBoothWindows :exec
DELETE FROM gpose_booth_windows WHERE booth_id = ?;

-- name: InsertGposeBoothWindow :exec
INSERT INTO gpose_booth_windows (booth_id, start_at, end_at, sort_order) VALUES (?, ?, ?, ?);

-- name: CountGposeSignups :one
SELECT COUNT(*) FROM gpose_signups WHERE booth_id = ?;

-- name: ListStallGposeBooths :many
-- The open booths pinned to a map's occupants, oldest first, so a pitch with more than
-- one keeps the newest (the stall panel links to one).
SELECT b.occupant_id, b.id, b.title
FROM gpose_booths b
JOIN festival_stall_occupants o ON o.id = b.occupant_id
JOIN festival_stalls st ON st.id = o.stall_id
WHERE st.map_id = ? AND b.occupant_id IS NOT NULL AND b.status = 'open'
ORDER BY b.created_at ASC, b.id ASC;

-- name: ClearMapGposeBooths :exec
UPDATE gpose_booths SET festival_map_id = NULL, occupant_id = NULL WHERE festival_map_id = ?;

-- name: ClearStallGposeOccupants :exec
UPDATE gpose_booths SET occupant_id = NULL WHERE occupant_id IN
  (SELECT id FROM festival_stall_occupants WHERE stall_id = ?);

-- name: ClearOccupantGposeBooths :exec
UPDATE gpose_booths SET occupant_id = NULL WHERE occupant_id = ?;

-- name: ListGposeSignups :many
SELECT id, booth_id, type_id, type_name, price, minutes, character_name, world, waitlisted, slot_start, slot_end,
       status, paid, called_at, created_at
FROM gpose_signups WHERE booth_id = ? ORDER BY created_at, id;

-- name: ListGposeSignupsFor :many
-- A character's sign-ups at a booth, on the whole name and world, ignoring case.
SELECT id, booth_id, type_id, type_name, price, minutes, character_name, world, waitlisted, slot_start, slot_end,
       status, paid, called_at, created_at
FROM gpose_signups
WHERE booth_id = sqlc.arg(booth_id) AND LOWER(character_name) = LOWER(sqlc.arg(name)) AND LOWER(world) = LOWER(sqlc.arg(world))
ORDER BY created_at, id;

-- name: GetGposeSignup :one
SELECT id, booth_id, type_id, type_name, price, minutes, character_name, world, waitlisted, slot_start, slot_end,
       status, paid, called_at, created_at
FROM gpose_signups WHERE id = ?;

-- name: InsertGposeSignup :one
INSERT INTO gpose_signups (booth_id, type_id, type_name, price, minutes, character_name, world, waitlisted,
                           slot_start, slot_end)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: CountGposeBooked :one
-- The sign-ups holding a place at a basic booth.
SELECT COUNT(*) FROM gpose_signups WHERE booth_id = ? AND waitlisted = 0 AND status IN ('waiting', 'completed');

-- name: CountGposeWaitlist :one
SELECT COUNT(*) FROM gpose_signups WHERE booth_id = ? AND waitlisted = 1 AND status = 'waiting';

-- name: CountGposePersonWaiting :one
SELECT COUNT(*) FROM gpose_signups
WHERE booth_id = sqlc.arg(booth_id) AND status = 'waiting'
  AND LOWER(character_name) = LOWER(sqlc.arg(name)) AND LOWER(world) = LOWER(sqlc.arg(world));

-- name: ListGposeHeldSlots :many
-- The booked slots at a timeslot booth that are still held.
SELECT id, CAST(slot_start AS TEXT) AS slot_start, CAST(slot_end AS TEXT) AS slot_end FROM gpose_signups
WHERE booth_id = ? AND slot_start IS NOT NULL AND waitlisted = 0 AND status IN ('waiting', 'completed');

-- name: FirstGposeWaitlisted :one
-- The longest-waiting person on a booth's waitlist.
SELECT id FROM gpose_signups WHERE booth_id = ? AND waitlisted = 1 AND status = 'waiting'
ORDER BY created_at, id LIMIT 1;

-- name: SetGposeSignupStatus :execrows
UPDATE gpose_signups SET status = ? WHERE id = ?;

-- name: SetGposeSignupPaid :execrows
UPDATE gpose_signups SET paid = ? WHERE id = ?;

-- name: SetGposeSignupWaitlisted :execrows
UPDATE gpose_signups SET waitlisted = ? WHERE id = ?;

-- name: SetGposeSignupSlot :execrows
-- Books a sign-up into a slot, taking it off the waitlist.
UPDATE gpose_signups SET slot_start = ?, slot_end = ?, waitlisted = 0 WHERE id = ?;

-- name: CallGposeSignup :execrows
-- Calls a waiting person up now.
UPDATE gpose_signups SET called_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = ? AND status = 'waiting';

-- name: DeleteGposeSignup :execrows
DELETE FROM gpose_signups WHERE id = ?;
