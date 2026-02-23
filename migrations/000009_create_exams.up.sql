-- Create exams table
CREATE TABLE IF NOT EXISTS exams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(500) NOT NULL,
    author_id INT NOT NULL REFERENCES admins(id) ON DELETE RESTRICT,
    scheduled_start TIMESTAMPTZ,
    scheduled_end TIMESTAMPTZ,
    duration_minutes INT NOT NULL DEFAULT 60,
    cheat_rules JSONB DEFAULT '{}'::jsonb NOT NULL,
    question_count INT DEFAULT 10 NOT NULL,
    randomize_questions BOOLEAN DEFAULT TRUE NOT NULL,
    entry_token VARCHAR(20),
    qbank_id UUID REFERENCES question_banks(id) ON DELETE RESTRICT,
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT'
        CHECK (status IN ('DRAFT', 'PUBLISHED', 'IN_PROGRESS', 'COMPLETED', 'ARCHIVED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_exams_author_id ON exams(author_id);
CREATE INDEX IF NOT EXISTS idx_exams_status ON exams(status);
CREATE INDEX IF NOT EXISTS idx_exams_qbank_id ON exams(qbank_id);

