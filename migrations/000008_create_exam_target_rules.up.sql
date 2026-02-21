-- Create exam_target_rules table
CREATE TABLE IF NOT EXISTS exam_target_rules (
    id SERIAL PRIMARY KEY,
    exam_id UUID NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
    target_type VARCHAR(10) NOT NULL CHECK (target_type IN ('CLASS', 'GRADE', 'MAJOR')),
    target_value VARCHAR(50) NOT NULL
);

-- Composite index for fast target lookups
CREATE INDEX IF NOT EXISTS idx_exam_target_rules_exam_type
    ON exam_target_rules(exam_id, target_type);
