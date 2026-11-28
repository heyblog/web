-- +goose Up

ALTER TABLE directory.sites DROP CONSTRAINT sites_blog_status_check;
ALTER TABLE directory.sites
    ADD CONSTRAINT sites_blog_status_check CHECK (blog_status IN ('UNREVIEWED', 'BLOG', 'HYBRID', 'NON_BLOG'));

COMMENT ON COLUMN directory.sites.blog_status IS 'Whether the site is an eligible blog, a hybrid/borderline blog, a confirmed non-blog, or awaiting annotation.'; -- Whether the site is an eligible blog, a hybrid/borderline blog, a confirmed non-blog, or awaiting annotation.

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Cannot downgrade site blog_status constraint; restore the previous database backup.'; END $$;
-- +goose StatementEnd
