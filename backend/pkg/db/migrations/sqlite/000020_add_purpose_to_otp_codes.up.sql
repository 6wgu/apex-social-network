CREATE TABLE otp_codes_new (
    purpose      TEXT     NOT NULL CHECK (purpose IN ('verify_email', 'reset_password')),
    subject      TEXT     NOT NULL,
    code_hash    TEXT     NOT NULL,
    salt         TEXT     NOT NULL,
    expires_at   DATETIME NOT NULL,
    attempts     INTEGER  NOT NULL DEFAULT 0,
    last_sent_at DATETIME NOT NULL,
    PRIMARY KEY (purpose, subject)
);

INSERT INTO otp_codes_new (purpose, subject, code_hash, salt, expires_at, attempts, last_sent_at)
SELECT 'verify_email', subject, code_hash, salt, expires_at, attempts, last_sent_at FROM otp_codes;

DROP TABLE otp_codes;
ALTER TABLE otp_codes_new RENAME TO otp_codes;
