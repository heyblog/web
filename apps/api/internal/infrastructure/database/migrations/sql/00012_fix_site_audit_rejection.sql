-- +goose Up

ALTER TABLE directory.site_audits
    DROP CONSTRAINT site_audits_create_site_check,
    ADD CONSTRAINT site_audits_create_site_check CHECK (
        action <> 'CREATE' OR status <> 'APPROVED' OR site_id IS NOT NULL
    );

COMMENT ON COLUMN directory.site_audits.site_id IS 'Target site; a CREATE request requires a site only after approval.'; -- Target site; a CREATE request requires a site only after approval.

-- +goose Down

-- Keep the compatibility check and constraint replacement under the same lock.
LOCK TABLE directory.site_audits IN ACCESS EXCLUSIVE MODE;

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM directory.site_audits
         WHERE action = 'CREATE' AND status = 'REJECTED' AND site_id IS NULL
    ) THEN
        RAISE EXCEPTION 'Cannot roll back migration 00012: rejected CREATE audits without a site exist.'
            USING HINT = 'Keep migration 00012 applied to preserve these audit outcomes.';
    END IF;
END;
$$;
-- +goose StatementEnd

ALTER TABLE directory.site_audits
    DROP CONSTRAINT site_audits_create_site_check,
    ADD CONSTRAINT site_audits_create_site_check CHECK (
        action <> 'CREATE' OR status = 'PENDING' OR site_id IS NOT NULL
    );

COMMENT ON COLUMN directory.site_audits.site_id IS 'Target site; absent only while a CREATE request is pending.';
