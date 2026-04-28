ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_change_set_id_fkey;
DROP TABLE IF EXISTS change_set_variable_entries;
DROP TABLE IF EXISTS change_set_entries;
DROP TABLE IF EXISTS change_sets;