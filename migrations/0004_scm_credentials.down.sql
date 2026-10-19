-- 0004_scm_credentials.down.sql
DELETE FROM credential_types
WHERE namespace = 'aiwx'
  AND name IN (
    'GitHub Personal Access Token',
    'GitHub App',
    'Git Personal Access Token'
  );
