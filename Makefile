
drop_test_db:
	psql postgresql://postgres@localhost:5432/template1 -c "DROP DATABASE test_sesh"

create_test_db:
	psql postgresql://postgres@localhost:5432/template1 -c "CREATE DATABASE test_sesh"

migrate_test_db:
	psql postgresql://postgres@localhost:5432/test_sesh -f migrations/create_sessions_table.sql

 reset_test_db:
	make drop_test_db
	make create_test_db
	make migrate_test_db