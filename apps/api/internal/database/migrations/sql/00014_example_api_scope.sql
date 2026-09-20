-- +goose Up
ALTER TABLE identity.api_client_scopes
DROP CONSTRAINT api_client_scopes_scope_check;

ALTER TABLE identity.api_client_scopes
ADD CONSTRAINT api_client_scopes_scope_check CHECK (
    scope IN ('data_import.write', 'example.call')
);

-- +goose Down
UPDATE identity.api_clients AS client
   SET disabled_at = clock_timestamp()
 WHERE client.disabled_at IS NULL
   AND EXISTS (
       SELECT 1
         FROM identity.api_client_scopes AS client_scope
        WHERE client_scope.client_id = client.id
          AND client_scope.scope = 'example.call'
   )
   AND NOT EXISTS (
       SELECT 1
         FROM identity.api_client_scopes AS client_scope
        WHERE client_scope.client_id = client.id
          AND client_scope.scope IN ('data_import.write', 'sites.read')
   );

DELETE FROM identity.api_client_scopes
 WHERE scope = 'example.call';

ALTER TABLE identity.api_client_scopes
DROP CONSTRAINT api_client_scopes_scope_check;

ALTER TABLE identity.api_client_scopes
ADD CONSTRAINT api_client_scopes_scope_check CHECK (
    scope IN ('data_import.write', 'sites.read')
);
