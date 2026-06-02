-- SCIM provisioning removed: no upstream IdP pushes users into Aegis for the
-- consuming projects. (Was 0029_scim_user.)
DROP TABLE IF EXISTS aegis.scim_user;
