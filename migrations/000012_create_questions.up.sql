-- Create questions table
CREATE TABLE IF NOT EXISTS questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    qbank_id UUID NOT NULL REFERENCES question_banks(id) ON DELETE CASCADE,
    question_text TEXT NOT NULL,
    question_type VARCHAR(50) NOT NULL DEFAULT 'MULTIPLE_CHOICE',
    options JSONB NOT NULL DEFAULT '[]',
    correct_option VARCHAR(5) NOT NULL,
    order_num INT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_questions_qbank_id ON questions(qbank_id);