package store

// migrations upgrade the schema step by step; the database's user_version
// is the number of applied migrations. Append new steps, never edit old ones.
var migrations = []string{
	// 1: initial schema. Dates are ISO 8601 (YYYY-MM-DD), booleans 0/1,
	// optional references NULL.
	`
CREATE TABLE users (
	id       TEXT PRIMARY KEY,
	name     TEXT NOT NULL,
	birthday TEXT NOT NULL DEFAULT ''
);

CREATE TABLE clubs (
	id   TEXT PRIMARY KEY,
	name TEXT NOT NULL
);

CREATE TABLE teams (
	id   TEXT PRIMARY KEY,
	name TEXT NOT NULL
);

CREATE TABLE competitions (
	id               TEXT PRIMARY KEY,
	name             TEXT NOT NULL,
	date             TEXT NOT NULL,
	shot_count       INTEGER NOT NULL,
	shots_per_target INTEGER NOT NULL,
	target_type      TEXT NOT NULL,
	decimal          INTEGER NOT NULL,
	active           INTEGER NOT NULL,
	modus            TEXT NOT NULL
);

-- League pairing of a league competition (1:1).
CREATE TABLE leagues (
	id             TEXT PRIMARY KEY,
	competition_id TEXT NOT NULL UNIQUE REFERENCES competitions (id) ON DELETE CASCADE,
	discipline     TEXT NOT NULL,
	league         TEXT NOT NULL,
	group_name     TEXT NOT NULL,
	home_club      TEXT REFERENCES clubs (id),
	home_team      TEXT REFERENCES teams (id),
	guest_club     TEXT REFERENCES clubs (id),
	guest_team     TEXT REFERENCES teams (id)
);

-- seq keeps the recording order, which is the entry order of competitions.
CREATE TABLE matches (
	seq            INTEGER PRIMARY KEY,
	id             TEXT NOT NULL UNIQUE,
	shooter_id     TEXT NOT NULL REFERENCES users (id),
	competition_id TEXT REFERENCES competitions (id),
	club_id        TEXT REFERENCES clubs (id),
	team_id        TEXT REFERENCES teams (id),
	target_type    TEXT NOT NULL,
	date           TEXT NOT NULL,
	active         INTEGER NOT NULL
);
CREATE INDEX matches_competition ON matches (competition_id, seq);
CREATE INDEX matches_shooter ON matches (shooter_id);

CREATE TABLE shots (
	match_id TEXT NOT NULL REFERENCES matches (id) ON DELETE CASCADE,
	idx      INTEGER NOT NULL,
	ringe    REAL NOT NULL,
	teiler   REAL NOT NULL,
	x        REAL NOT NULL,
	y        REAL NOT NULL,
	PRIMARY KEY (match_id, idx)
) WITHOUT ROWID;
`,
	// 2: title and scoring of quick analyses, which have no competition.
	`
ALTER TABLE matches ADD COLUMN title TEXT NOT NULL DEFAULT '';
ALTER TABLE matches ADD COLUMN decimal INTEGER NOT NULL DEFAULT 0;
`,
}
