-- Create student_room_assignments table
CREATE TABLE IF NOT EXISTS student_room_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_session_id UUID NOT NULL REFERENCES room_sessions(id) ON DELETE CASCADE,
    student_id INT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    seat_number INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (room_session_id, student_id),
    UNIQUE (student_id)  -- a student can only be in ONE room session globally
);

CREATE INDEX IF NOT EXISTS idx_student_room_assignments_session_id ON student_room_assignments(room_session_id);
CREATE INDEX IF NOT EXISTS idx_student_room_assignments_student_id ON student_room_assignments(student_id);
