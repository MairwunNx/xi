#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "ximanager" --dbname "ximanager" <<-EOSQL
    SELECT 'CREATE DATABASE unleash'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'unleash')\gexec
    GRANT ALL PRIVILEGES ON DATABASE unleash TO "ximanager";
EOSQL