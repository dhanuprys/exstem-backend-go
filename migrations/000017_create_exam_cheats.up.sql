CREATE TABLE IF NOT EXISTS exam_cheats (
    id SERIAL PRIMARY KEY,
    exam_id UUID NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
    student_id INT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    event_data JSONB NOT NULL,
    recorded_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_exam_cheats_exam_id ON exam_cheats(exam_id);
CREATE INDEX IF NOT EXISTS idx_exam_cheats_student_id ON exam_cheats(student_id);
