CREATE TABLE sessions(
    session_key     text PRIMARY KEY,
    account_id      text UNIQUE NOT NULL,
    expiration_date timestamp NOT NULL
);