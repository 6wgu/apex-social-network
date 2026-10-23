CREATE TABLE otp_codes_old (
    subject      TEXT     PRIMARY KEY,
    code_hash    TEXT     NOT NULL,
    salt         TEXT     NOT NULL,
    expires_at   DATETIME NOT NULL,
    attempts     INTEGER  NOT NULL DEFAULT 0,
    last_sent_at DATETIME NOT NULL
);

INSERT INTO otp_codes_old (subject, code_hash, salt, expires_at, attempts, last_sent_at)
SELECT subject, code_hash, salt, expires_at, attempts, last_sent_at FROM otp_codes WHERE purpose = 'verify_email';

DROP TABLE otp_codes;
ALTER TABLE otp_codes_old RENAME TO otp_codes;
