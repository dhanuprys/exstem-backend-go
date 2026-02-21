-- Create exam_sessions table
CREATE TABLE IF NOT EXISTS exam_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_id UUID NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
    student_id INT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL DEFAULT 'IN_PROGRESS'
        CHECK (status IN ('IN_PROGRESS', 'COMPLETED')),
    final_score DECIMAL(5,2),
    UNIQUE (exam_id, student_id)
);

-- Composite index for fast session lookups
CREATE INDEX IF NOT EXISTS idx_exam_sessions_student_exam
    ON exam_sessions(student_id, exam_id);
