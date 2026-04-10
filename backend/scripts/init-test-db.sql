SELECT 'CREATE DATABASE taskflow_test'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'taskflow_test')\gexec
