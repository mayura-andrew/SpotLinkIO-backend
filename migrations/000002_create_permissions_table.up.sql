CREATE TABLE IF NOT EXISTS permissions (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    code text NOT NULL
);

CREATE TABLE IF NOT EXISTS users_permissions (
    user_id UUID NOT NULL REFERENCES users ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions ON DELETE CASCADE,
    PRIMARY KEY (user_id, permission_id)
);

INSERT INTO permissions (id, code) VALUES (gen_random_uuid(), 'ideas:read'),
(gen_random_uuid(), 'ideas:write');