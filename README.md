# Sesh

Sesh is a library for using a session table + session cookie to manage sesssions in a go server.

## Dev Setup

1. start a postgres server
2. create a .env file with `cp .env.example .env`, set values to match your postgres config
3. run `make reset_test_db`
4. run `make test`
