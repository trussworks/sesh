
# import .env vars
include .env
export $(shell sed 's/=.*//' .env)

db_base = postgresql://$$DATABASE_USER@$$DATABASE_HOST:$$DATABASE_PORT
db_url = $(db_base)/$$DATABASE_NAME

help:
	echo "Test with `make test`, setup the db with `make reset_test_db`"

drop_test_db:
	psql $(db_base)/template1 -c "DROP DATABASE test_sesh"

create_test_db:
	psql $(db_base)/template1 -c "CREATE DATABASE test_sesh"

load_db_schema:
	psql $(db_url) -f migrations/create_sessions_table.sql
	psql $(db_url) -f migrations/create_user_tablef.sql

 reset_test_db:
	make drop_test_db || true
	make create_test_db
	make load_db_schema

test:
	go test ./...
