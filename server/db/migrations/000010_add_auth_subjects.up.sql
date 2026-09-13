ALTER TABLE bookers ADD COLUMN subject TEXT;
CREATE UNIQUE INDEX uq_bookers_subject ON bookers (subject) WHERE subject IS NOT NULL;

ALTER TABLE accommodations ADD COLUMN operator_subject TEXT;
CREATE INDEX idx_accommodations_operator ON accommodations (operator_subject);
