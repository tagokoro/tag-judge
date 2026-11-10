DROP TABLE IF EXISTS library_submission_links;
DROP TRIGGER IF EXISTS library_versions_no_update_delete ON library_versions;
DROP FUNCTION IF EXISTS reject_library_version_mutation();
DROP TABLE IF EXISTS library_versions;
DROP TABLE IF EXISTS libraries;
DROP TABLE IF EXISTS namespaces;
