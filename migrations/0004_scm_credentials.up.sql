-- 0004_scm_credentials.up.sql
-- SCM authentication credential types for project sync.

INSERT INTO credential_types (name, namespace, inputs_schema, injectors) VALUES
    (
        'GitHub Personal Access Token',
        'aiwx',
        '{
            "username": {"type": "string", "help_text": "Defaults to x-access-token when empty"},
            "token": {"type": "string", "secret": true, "help_text": "GitHub PAT (classic or fine-grained) with repo read"}
        }',
        '{"env": {"GIT_USERNAME": "{{ username }}", "GIT_TOKEN": "{{ token }}"}}'
    ),
    (
        'GitHub App',
        'aiwx',
        '{
            "app_id": {"type": "string", "help_text": "GitHub App ID"},
            "installation_id": {"type": "string", "help_text": "Installation ID on the org/user"},
            "private_key": {"type": "string", "secret": true, "help_text": "PEM-encoded RSA private key"},
            "base_url": {"type": "string", "help_text": "Optional GitHub Enterprise API base, default https://api.github.com"}
        }',
        '{}'
    ),
    (
        'Git Personal Access Token',
        'aiwx',
        '{
            "username": {"type": "string", "help_text": "Defaults to git when empty"},
            "token": {"type": "string", "secret": true, "help_text": "Token for HTTPS git clone"}
        }',
        '{"env": {"GIT_USERNAME": "{{ username }}", "GIT_TOKEN": "{{ token }}"}}'
    )
ON CONFLICT (namespace, name) DO NOTHING;
