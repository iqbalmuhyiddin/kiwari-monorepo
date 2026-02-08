#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
    CREATE USER pos_stg WITH PASSWORD '${POS_STG_PASSWORD}';
    CREATE DATABASE pos_staging OWNER pos_stg;
    GRANT ALL PRIVILEGES ON DATABASE pos_staging TO pos_stg;

    CREATE USER pos_prod WITH PASSWORD '${POS_PROD_PASSWORD}';
    CREATE DATABASE pos_production OWNER pos_prod;
    GRANT ALL PRIVILEGES ON DATABASE pos_production TO pos_prod;
EOSQL
