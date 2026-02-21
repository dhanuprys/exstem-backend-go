-- Create questions table
CREATE TABLE IF NOT EXISTS questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_id UUID NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
    question_text TEXT NOT NULL,
    options JSONB NOT NULL DEFAULT '[]',
    correct_option VARCHAR(5) NOT NULL,
    order_num INT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_questions_exam_id ON questions(exam_id);
