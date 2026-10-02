-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'age') THEN
        RAISE EXCEPTION 'AGE extension is missing; provision it with the postgres bootstrap role';
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
SELECT 1;
