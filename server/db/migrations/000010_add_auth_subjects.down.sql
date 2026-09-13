DROP INDEX idx_accommodations_operator;
ALTER TABLE accommodations DROP COLUMN operator_subject;
DROP INDEX uq_bookers_subject;
ALTER TABLE bookers DROP COLUMN subject;
