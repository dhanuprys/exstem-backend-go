-- Create room_sessions table (standalone, not tied to any exam)
CREATE TABLE IF NOT EXISTS room_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_number INT NOT NULL,
    room_id INT NOT NULL REFERENCES rooms(id) ON DELETE RESTRICT,
    start_time TIME,
    end_time TIME,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (session_number, room_id)
);

CREATE INDEX IF NOT EXISTS idx_room_sessions_room_id ON room_sessions(room_id);
