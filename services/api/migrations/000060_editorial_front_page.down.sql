DROP TABLE IF EXISTS editorials;
DROP TABLE IF EXISTS front_page_placements;
ALTER TABLE sources DROP COLUMN IF EXISTS front_page;
UPDATE admin_users SET role = 'editor' WHERE role = 'chief_editor';
ALTER TABLE admin_users DROP CONSTRAINT admin_users_role_valid;
ALTER TABLE admin_users ADD CONSTRAINT admin_users_role_valid CHECK (role IN (
  'administrator', 'source_manager', 'editor', 'compliance_reviewer', 'operations_engineer'
));
