-- Create exam_room_assignments table
CREATE TABLE IF NOT EXISTS exam_room_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_schedule_id UUID NOT NULL REFERENCES exam_schedules(id) ON DELETE CASCADE,
    student_id INT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    seat_number INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (exam_schedule_id, student_id)
);

CREATE INDEX IF NOT EXISTS idx_exam_room_assignments_schedule_id ON exam_room_assignments(exam_schedule_id);
CREATE INDEX IF NOT EXISTS idx_exam_room_assignments_student_id ON exam_room_assignments(student_id);
