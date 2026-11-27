-- +goose Up
-- Gpose booths: a photographer's booth that people sign up for, filed under a festival
-- and pinned to one of its stalls when it is part of one (both plain columns, as a
-- raffle's are, so deleting the festival or the stall clears them in code).
--
-- The kinds of shoot (Solo, Couple, Group...) are one list for the whole site. Each
-- booth picks which of them it offers, with its own price and, for a timeslot booth,
-- how long one takes.
CREATE TABLE gpose_types (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')) CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at))
);
CREATE UNIQUE INDEX idx_gpose_types_name ON gpose_types(name COLLATE NOCASE);

-- A booth. mode 'basic' takes sign-ups in order up to max_signups, then a waitlist of
-- up to max_waitlist; mode 'timeslot' books start times every slot_step minutes inside
-- its windows, with a waitlist of up to max_waitlist beside them. per_person is the
-- most sign-ups one character may have waiting at once.
CREATE TABLE gpose_booths (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  mode TEXT NOT NULL CHECK (mode IN ('basic', 'timeslot')),
  max_signups INTEGER NOT NULL DEFAULT 1,
  max_waitlist INTEGER NOT NULL DEFAULT 0,
  per_person INTEGER NOT NULL DEFAULT 1,
  slot_step INTEGER NOT NULL DEFAULT 15,
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
  festival_map_id INTEGER,
  occupant_id INTEGER,
  created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')) CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at))
);

-- The types a booth offers. A type a booth offers can't be deleted from the site's list.
CREATE TABLE gpose_booth_types (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  booth_id INTEGER NOT NULL REFERENCES gpose_booths(id) ON DELETE CASCADE,
  type_id INTEGER NOT NULL REFERENCES gpose_types(id),
  price INTEGER NOT NULL DEFAULT 0,
  minutes INTEGER NOT NULL DEFAULT 0,
  sort_order INTEGER NOT NULL DEFAULT 0,
  UNIQUE (booth_id, type_id)
);

-- When a timeslot booth is open; never overlapping.
CREATE TABLE gpose_booth_windows (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  booth_id INTEGER NOT NULL REFERENCES gpose_booths(id) ON DELETE CASCADE,
  start_at DATETIME NOT NULL CHECK (start_at IS strftime('%Y-%m-%dT%H:%M:%SZ', start_at)),
  end_at DATETIME NOT NULL CHECK (end_at IS strftime('%Y-%m-%dT%H:%M:%SZ', end_at)),
  sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_gpose_booth_windows_booth ON gpose_booth_windows(booth_id);

-- A sign-up. The type's name, price and length are kept as they were, so editing the
-- booth or the type later rewrites nothing already booked. A waitlisted sign-up has no
-- slot; a booked one at a timeslot booth has one. waiting holds a place or a slot, and
-- so does completed; no_show and canceled give it up. called_at is when staff last
-- called the person up, which alerts them on the festival dashboard until they
-- dismiss it.
CREATE TABLE gpose_signups (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  booth_id INTEGER NOT NULL REFERENCES gpose_booths(id) ON DELETE CASCADE,
  type_id INTEGER REFERENCES gpose_types(id) ON DELETE SET NULL,
  type_name TEXT NOT NULL,
  price INTEGER NOT NULL DEFAULT 0,
  minutes INTEGER NOT NULL DEFAULT 0,
  character_name TEXT NOT NULL,
  world TEXT NOT NULL,
  waitlisted INTEGER NOT NULL DEFAULT 0,
  slot_start DATETIME CHECK (slot_start IS strftime('%Y-%m-%dT%H:%M:%SZ', slot_start)),
  slot_end DATETIME CHECK (slot_end IS strftime('%Y-%m-%dT%H:%M:%SZ', slot_end)),
  status TEXT NOT NULL DEFAULT 'waiting' CHECK (status IN ('waiting', 'completed', 'no_show', 'canceled')),
  paid INTEGER NOT NULL DEFAULT 0,
  called_at DATETIME CHECK (called_at IS strftime('%Y-%m-%dT%H:%M:%SZ', called_at)),
  created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')) CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
  CHECK ((slot_start IS NULL) = (slot_end IS NULL))
);
CREATE INDEX idx_gpose_signups_booth ON gpose_signups(booth_id, status);
-- Two bookings can't start at the same moment at one booth: the backstop beneath the
-- overlap check, which runs inside the write lock.
CREATE UNIQUE INDEX idx_gpose_signups_slot ON gpose_signups(booth_id, slot_start)
  WHERE slot_start IS NOT NULL AND waitlisted = 0 AND status IN ('waiting', 'completed');
PRAGMA user_version = 79;
