-- Create permissions table
CREATE TABLE IF NOT EXISTS permissions (
    id SERIAL PRIMARY KEY,
    code VARCHAR(100) NOT NULL UNIQUE,
    description VARCHAR(255) NOT NULL DEFAULT ''
);

-- Seed default permissions
INSERT INTO permissions (code, description) VALUES
    ('exams:read', 'View exams'),
    ('exams:write_own', 'Create and edit own exams'),
    ('exams:write_all', 'Create and edit all exams'),
    ('exams:publish', 'Publish exams'),
    ('students:read', 'View students'),
    ('students:write', 'Create and edit students'),
    ('students:reset_session', 'Reset student login sessions'),
    ('classes:read', 'View classes'),
    ('classes:write', 'Create and edit classes'),
    ('media:upload', 'Upload media files'),
    ('roles:read', 'View admin roles'),
    ('roles:write', 'Create, edit, and delete admin roles'),
    ('admins:read', 'View admin users'),
    ('admins:write', 'Create, edit, and delete admin users'),
    ('settings:read', 'View application settings'),
    ('settings:write', 'Edit application settings'),
    ('subjects:read', 'View subjects'),
    ('subjects:write', 'Create, edit, and delete subjects')
ON CONFLICT (code) DO NOTHING;
