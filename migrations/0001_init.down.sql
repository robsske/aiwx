-- AIWX initial schema rollback
-- Migration: 0001_init.down.sql

DROP TABLE IF EXISTS service_accounts CASCADE;
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS outbox CASCADE;
DROP TABLE IF EXISTS job_events CASCADE;
DROP TABLE IF EXISTS jobs CASCADE;
DROP TABLE IF EXISTS job_template_credentials CASCADE;
DROP TABLE IF EXISTS job_templates CASCADE;
DROP TABLE IF EXISTS execution_environments CASCADE;
DROP TABLE IF EXISTS inventory_sources CASCADE;
DROP TABLE IF EXISTS inventory_group_hosts CASCADE;
DROP TABLE IF EXISTS inventory_hosts CASCADE;
DROP TABLE IF EXISTS inventory_groups CASCADE;
DROP TABLE IF EXISTS inventories CASCADE;
DROP TABLE IF EXISTS projects CASCADE;
DROP TABLE IF EXISTS credentials CASCADE;
DROP TABLE IF EXISTS credential_types CASCADE;
DROP TABLE IF EXISTS role_assignments CASCADE;
DROP TABLE IF EXISTS roles CASCADE;
DROP TABLE IF EXISTS team_members CASCADE;
DROP TABLE IF EXISTS teams CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS organizations CASCADE;

DROP POLICY IF EXISTS tenant_isolation_service_accounts ON service_accounts;
DROP POLICY IF EXISTS tenant_isolation_audit_logs ON audit_logs;
DROP POLICY IF EXISTS tenant_isolation_job_events ON job_events;
DROP POLICY IF EXISTS tenant_isolation_jobs ON jobs;
DROP POLICY IF EXISTS tenant_isolation_job_templates ON job_templates;
DROP POLICY IF EXISTS tenant_isolation_inventory_sources ON inventory_sources;
DROP POLICY IF EXISTS tenant_isolation_inventory_group_hosts ON inventory_group_hosts;
DROP POLICY IF EXISTS tenant_isolation_inventory_hosts ON inventory_hosts;
DROP POLICY IF EXISTS tenant_isolation_inventory_groups ON inventory_groups;
DROP POLICY IF EXISTS tenant_isolation_inventories ON inventories;
DROP POLICY IF EXISTS tenant_isolation_projects ON projects;
DROP POLICY IF EXISTS tenant_isolation_credentials ON credentials;
DROP POLICY IF EXISTS tenant_isolation_team_members ON team_members;
DROP POLICY IF EXISTS tenant_isolation_teams ON teams;

DROP ROLE IF EXISTS aiwx_app;
