//go:build integration

package integration_test

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func seedDatabaseBackupOwnership(t *testing.T, fixture auditMigrationFixture, actor string, siteID pgtype.UUID) {
	t.Helper()
	ctx := t.Context()
	statements := []string{
		`INSERT INTO directory.site_claims(site_id,user_id,address,method,status,evidence,reviewed_by,created_at,verified_at,consumed_at)
 SELECT $2,id,'https://backup.example.test/','MANUAL',generated_status,'Historical proof',$1,'2024-01-01'::timestamptz,'2024-01-02'::timestamptz,'2024-01-03'::timestamptz
 FROM identity.users CROSS JOIN unnest(ARRAY['PENDING','VERIFIED','REJECTED','CANCELLED']) generated_status WHERE role='ADMIN'`,
		`INSERT INTO directory.site_ownerships(site_id,user_id,address,revision,created_at,updated_at)
 VALUES($2,$1,'https://backup.example.test/',9007199254740993,'2024-01-01','2024-01-03')`,
		`INSERT INTO directory.site_ownership_events(site_id,user_id,actor_id,action,reason,evidence,created_at)
 SELECT $2,$1,$1,action,'Historical decision','Synthetic evidence','2024-01-04'::timestamptz
 FROM unnest(ARRAY['VERIFIED','REVOKED','REASSIGNED','ADDRESS_CHANGED']) action`,
		`INSERT INTO directory.site_audits(lookup_secret_hash,action,status,site_id,base_revision,base_snapshot,proposed_snapshot,request_reason,submitter_user_id,source_channel,source_site_id,ownership_id,created_at,updated_at)
 SELECT decode(repeat('ef',32),'hex'),'UPDATE','PENDING',$2,1,'{}',jsonb_build_object('scheme','https','normalized_host','backup.example.test','base_path','/'),'Historical owner update',$1,'OWNER_UPDATE',$2,id,'2024-02-01','2024-02-02' FROM directory.site_ownerships`,
		`INSERT INTO directory.site_audits(lookup_secret_hash,action,proposed_snapshot,request_reason,submitter_user_id,source_channel,created_at,updated_at)
 VALUES(decode(repeat('aa',32),'hex'),'CREATE','{}','Historical account submission',$1,'ACCOUNT_SUBMISSION','2024-02-03','2024-02-04')`,
		`INSERT INTO directory.owner_friend_link_requests(audit_id,source_site_id,user_id,ownership_id,status,notify_by_email,created_at,updated_at)
 SELECT a.id,$2,$1,uuidv7(),generated_status,true,'2024-02-05'::timestamptz,'2024-02-06'::timestamptz
 FROM directory.site_audits a CROSS JOIN unnest(ARRAY['PENDING','APPLIED','CANCELLED','REJECTED']) generated_status WHERE a.source_channel='ACCOUNT_SUBMISSION'`,
		`INSERT INTO directory.site_sources(source_key,name) VALUES('OWNER_FRIEND_LINK','Owner recommendation')`,
		`INSERT INTO directory.site_origins(site_id,source_id,external_reference,metadata)
 SELECT $2,id,'synthetic',jsonb_build_object('channel','OWNER_FRIEND_LINK','user_id',$1::text,'recommendations',jsonb_build_array(jsonb_build_object('user_id',$1::text,'source_site_id',$2::text)),'opaque',jsonb_build_object('user_id',$1::text)) FROM directory.site_sources WHERE source_key='OWNER_FRIEND_LINK'`,
		// Captured generations remain historical after reassignment.
		`DELETE FROM directory.site_ownerships WHERE site_id=$2`,
		`INSERT INTO directory.site_ownerships(site_id,user_id,address,revision,created_at,updated_at)
 VALUES($2,$1,'https://backup.example.test/',7,'2024-03-01'::timestamptz,'2024-03-02'::timestamptz)`,
	}
	for _, statement := range statements {
		if _, err := fixture.admin.Exec(ctx, `SELECT set_config('backup.actor',$1,false),set_config('backup.site',$2,false)`, actor, integrationUUIDText(t, siteID)); err != nil {
			t.Fatal(err)
		}
		statement = strings.NewReplacer("$1", "current_setting('backup.actor')::uuid", "$2", "current_setting('backup.site')::uuid").Replace(statement)
		if _, err := fixture.admin.Exec(ctx, statement); err != nil {
			t.Fatalf("seed ownership backup: %v", err)
		}
	}
}
