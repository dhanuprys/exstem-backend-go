-- Create app_settings table
CREATE TABLE IF NOT EXISTS app_settings (
    key VARCHAR(100) PRIMARY KEY,
    value TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default settings
INSERT INTO app_settings (key, value) VALUES
    ('school_name', 'Nama Sekolah'),
    ('school_location', 'Lokasi Sekolah'),
    ('school_logo_url', '')
ON CONFLICT (key) DO NOTHING;
