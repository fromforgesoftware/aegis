-- Org-scoped groups (Teams): a group may belong to an organization so its
-- bindings grant access scoped to that tenant. NULL = realm-level group.
ALTER TABLE aegis.actor_set
    ADD COLUMN organization_id UUID REFERENCES aegis.organization(id) ON DELETE CASCADE;
CREATE INDEX idx_actor_set_organization ON aegis.actor_set (organization_id);
