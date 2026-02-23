-- Create student_answers table for persisting autosaved answers
CREATE TABLE IF NOT EXISTS student_answers (
    id SERIAL PRIMARY KEY,
    exam_id UUID NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
    student_id INT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    answer VARCHAR(5) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (exam_id, student_id, question_id)
);

CREATE INDEX IF NOT EXISTS idx_student_answers_exam_student
    ON student_answers(exam_id, student_id);
