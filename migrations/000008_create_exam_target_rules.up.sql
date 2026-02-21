-- Create exam_target_rules table
CREATE TABLE IF NOT EXISTS exam_target_rules (
    id SERIAL PRIMARY KEY,
    exam_id UUID NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
    class_id INT REFERENCES classes(id) ON DELETE SET NULL,
    grade_level VARCHAR(10),
    major_code VARCHAR(20),
    religion VARCHAR(50)
);

-- Index for fast target lookups
CREATE INDEX IF NOT EXISTS idx_exam_target_rules_exam_id
    ON exam_target_rules(exam_id);
