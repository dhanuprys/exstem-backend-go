-- Create role_permissions join table
CREATE TABLE IF NOT EXISTS role_permissions (
    role_id INT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id INT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- Grant all permissions to Superadmin
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'Superadmin'
ON CONFLICT DO NOTHING;

-- Grant teacher-level permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'Teacher'
  AND p.code IN (
    'qbanks:read', 'qbanks:write_own',
    'classes:read', 'media:upload'
  )
ON CONFLICT DO NOTHING;