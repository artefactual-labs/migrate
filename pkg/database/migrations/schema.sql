-- schema
CREATE TABLE IF NOT EXISTS aips (
    id                          INTEGER PRIMARY KEY,
    uuid                        TEXT NOT NULL UNIQUE CHECK (LENGTH(uuid) == 36),
    status                      TEXT NOT NULL DEFAULT 'new',
    found                       BOOLEAN NOT NULL DEFAULT FALSE,
    fixity_run                  BOOLEAN NOT NULL DEFAULT FALSE,
    moved                       BOOLEAN NOT NULL DEFAULT FALSE,
    cleaned                     BOOLEAN NOT NULL DEFAULT FALSE,
    replicated                  BOOLEAN NOT NULL DEFAULT FALSE,
    re_indexed                  BOOLEAN NOT NULL DEFAULT FALSE,
    current_location            TEXT DEFAULT '',
    "size"                      UNSIGNED BIG INT,
    location_uuid               TEXT
);

CREATE INDEX IF NOT EXISTS aips_uuid_idx ON aips ("uuid");

CREATE TABLE IF NOT EXISTS errors (
    id          INTEGER PRIMARY KEY,
    aip_id      INTEGER NOT NULL,
    msg         TEXT NOT NULL,
    details     TEXT,

    FOREIGN KEY (aip_id) REFERENCES aips (id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS events (
    id              INTEGER PRIMARY KEY,
    aip_id          INTEGER NOT NULL,

    action                      TEXT NOT NULL,
    time_started                TEXT NOT NULL,
    time_ended                  TEXT NOT NULL,
    total_duration              TEXT,
    total_duration_nanoseconds  BIGINT,
    details                     TEXT,

    FOREIGN KEY (aip_id) REFERENCES aips (id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS aip_replication (
    id              INTEGER PRIMARY KEY,
    aip_id          INTEGER NOT NULL,

    location_uuid   TEXT DEFAULT '',
    replica_uuid    TEXT DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'new',
    attempt         INTEGER NOT NULL DEFAULT 0,

    FOREIGN KEY (aip_id) REFERENCES aips (id) ON DELETE CASCADE
);