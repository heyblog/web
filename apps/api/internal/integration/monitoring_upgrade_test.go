//go:build integration

package integration_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMonitoringUpgradeFromDeployedBaseline(t *testing.T) {
	// Given deployed schema 8 with business data and a previous access conclusion.
	upgraded := newAuditMigrationFixture(t)
	_, err := upgraded.provider.UpTo(t.Context(), 20)
	require.NoError(t, err)
	var id string
	var revision int64
	var updated time.Time
	require.NoError(t, upgraded.pool.QueryRow(t.Context(), `INSERT INTO directory.sites
		(short_id,name,scheme,normalized_host,base_path,access_scope,visibility)
		VALUES('Upgrade01','Deployed content','https','upgrade.example.test','/','CN_ONLY','VISIBLE')
		RETURNING id::text,revision,updated_at`).Scan(&id, &revision, &updated))
	// When the remaining migrations are applied.
	results, err := upgraded.provider.Up(t.Context())
	require.NoError(t, err)
	require.Len(t, results, 4)
	var name string
	var scope, status *string
	var actualRevision int64
	var actualUpdated time.Time
	require.NoError(t, upgraded.pool.QueryRow(t.Context(), `SELECT name,access_scope,status,revision,updated_at
		FROM directory.sites WHERE id=$1`, id).Scan(&name, &scope, &status, &actualRevision, &actualUpdated))
	require.Equal(t, "Deployed content", name)
	require.Nil(t, scope)
	require.Nil(t, status)
	require.Equal(t, revision, actualRevision)
	require.Equal(t, updated, actualUpdated)
	var datasets, addedColumns int
	require.NoError(t, upgraded.pool.QueryRow(t.Context(), `SELECT count(DISTINCT dataset),
		count(*) FILTER (WHERE dataset LIKE 'operations.%' OR column_name IN ('monitoring_generation','last_checked_at'))
		FROM directory.backup_schema8_columns()`).Scan(&datasets, &addedColumns))
	require.Equal(t, 35, datasets)
	require.Zero(t, addedColumns)
	// Then fresh initialization has the same columns, constraints, indexes, grants and seed semantics.
	fresh := newAuditMigrationFixture(t)
	_, err = fresh.provider.Up(t.Context())
	require.NoError(t, err)
	require.Equal(t, monitoringSchemaSnapshot(t, fresh), monitoringSchemaSnapshot(t, upgraded))
	results, err = upgraded.provider.Up(t.Context())
	require.NoError(t, err)
	require.Empty(t, results)
	version, err := upgraded.provider.GetDBVersion(t.Context())
	require.NoError(t, err)
	require.EqualValues(t, 24, version)
}

func monitoringSchemaSnapshot(t *testing.T, fixture auditMigrationFixture) string {
	t.Helper()
	var snapshot string
	require.NoError(t, fixture.admin.QueryRow(t.Context(), `SELECT jsonb_build_object(
		'columns',(SELECT jsonb_agg(to_jsonb(c) ORDER BY table_schema,table_name,ordinal_position)
		 FROM information_schema.columns c WHERE table_schema IN ('identity','directory','content','operations')),
		'constraints',(SELECT jsonb_agg(jsonb_build_array(n.nspname,c.relname,k.conname,pg_get_constraintdef(k.oid)) ORDER BY n.nspname,c.relname,k.conname)
		 FROM pg_constraint k JOIN pg_class c ON c.oid=k.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace
		 WHERE n.nspname IN ('identity','directory','content','operations')),
		'indexes',(SELECT jsonb_agg(to_jsonb(i) ORDER BY schemaname,tablename,indexname) FROM pg_indexes i
		 WHERE schemaname IN ('identity','directory','content','operations')),
		'grants',(SELECT jsonb_agg(to_jsonb(g) ORDER BY table_schema,table_name,grantee,privilege_type) FROM information_schema.table_privileges g
		 WHERE table_schema IN ('identity','directory','content','operations')),
		'configs',(SELECT jsonb_agg(to_jsonb(c)-ARRAY['id','created_at'] ORDER BY purpose) FROM operations.request_configs c),
		'schedules',(SELECT jsonb_agg((to_jsonb(s)-ARRAY['id','created_at','request_config_id','configuration'])||
		 jsonb_build_object('configuration',configuration-'request_config_id') ORDER BY kind) FROM operations.schedules s),
		'bindings',(SELECT jsonb_agg(c.purpose ORDER BY s.id) FROM operations.task_settings s JOIN operations.request_configs c ON c.id=s.verification_config_id),
		'policy',(SELECT to_jsonb(p) FROM operations.signing_policy p),
		'backoff',(SELECT jsonb_agg(to_jsonb(b) ORDER BY platform) FROM operations.platform_backoff b),
		'protected',jsonb_build_array(
		 has_function_privilege('api_runtime','directory.backup_restore_begin(uuid)','EXECUTE'),
		 has_function_privilege('api_runtime','directory.backup_restore_begin_v8(uuid)','EXECUTE'),
		 has_function_privilege('api_runtime','directory.backup_restore_finish_v8()','EXECUTE'))
	)::text`).Scan(&snapshot))
	return snapshot
}
