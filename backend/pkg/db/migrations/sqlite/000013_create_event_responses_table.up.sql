-- One answer per member and event: going or not going. Answering again
-- replaces the earlier answer.
CREATE TABLE IF NOT EXISTS event_responses (
    event_id     INTEGER  NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id      INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    response     TEXT     NOT NULL CHECK (response IN ('going', 'not_going')),
    responded_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (event_id, user_id)
);
