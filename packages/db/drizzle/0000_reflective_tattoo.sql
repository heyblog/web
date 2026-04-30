CREATE TYPE "public"."announcement_status_enum" AS ENUM('DRAFT', 'SCHEDULED', 'PUBLISHED', 'EXPIRED');--> statement-breakpoint
CREATE TYPE "public"."article_feedback_action_enum" AS ENUM('HIDE', 'DELETE');--> statement-breakpoint
CREATE TYPE "public"."article_feedback_reason_enum" AS ENUM('CONTENT_ERROR', 'BROKEN_LINK', 'POLITICAL_SENSITIVE', 'PORNOGRAPHY_VIOLENCE', 'COPYRIGHT', 'SPAM', 'DUPLICATE', 'OTHER');--> statement-breakpoint
CREATE TYPE "public"."article_visibility_enum" AS ENUM('VISIBLE', 'HIDDEN', 'DELETED');--> statement-breakpoint
CREATE TYPE "public"."audit_status_enum" AS ENUM('PENDING', 'APPROVED', 'REJECTED', 'CANCELED');--> statement-breakpoint
CREATE TYPE "public"."content_validation_status_enum" AS ENUM('NOT_REQUESTED', 'PASSED', 'FAILED');--> statement-breakpoint
CREATE TYPE "public"."deployment_module_enum" AS ENUM('WEB', 'API', 'WORKER', 'DEPLOYER', 'STATUS', 'CLOUDFLARE', 'ALL', 'DB');--> statement-breakpoint
CREATE TYPE "public"."deployment_status_enum" AS ENUM('PENDING', 'RUNNING', 'SUCCESS', 'FAILED', 'ROLLED_BACK', 'SKIPPED');--> statement-breakpoint
CREATE TYPE "public"."feed_type_enum" AS ENUM('RSS', 'ATOM', 'JSON');--> statement-breakpoint
CREATE TYPE "public"."from_source_enum" AS ENUM('CIB', 'BO_YOU_QUAN', 'BLOG_FINDER', 'BKZ', 'TRAVELLINGS', 'WEB_SUBMIT', 'LINK_PAGE_SEARCH', 'OLD_DATA');--> statement-breakpoint
CREATE TYPE "public"."job_status_enum" AS ENUM('PENDING', 'RUNNING', 'SUCCEEDED', 'FAILED', 'CANCELED');--> statement-breakpoint
CREATE TYPE "public"."job_trigger_source_enum" AS ENUM('SCHEDULE', 'MANUAL', 'EVENT');--> statement-breakpoint
CREATE TYPE "public"."management_permission_enum" AS ENUM('user.manage', 'site_audit.review', 'feedback.review', 'announcement.manage', 'taxonomy.manage', 'site.manage', 'task.manage', 'log.read');--> statement-breakpoint
CREATE TYPE "public"."request_retry_strategy_enum" AS ENUM('FIXED', 'LINEAR', 'EXPONENTIAL');--> statement-breakpoint
CREATE TYPE "public"."request_target_kind_enum" AS ENUM('SITE', 'SITE_LIST', 'ALL_VISIBLE');--> statement-breakpoint
CREATE TYPE "public"."rss_feed_format_enum" AS ENUM('RSS', 'ATOM', 'JSON', 'UNKNOWN');--> statement-breakpoint
CREATE TYPE "public"."rss_feed_mode_enum" AS ENUM('DEFAULT_ONLY', 'ALL');--> statement-breakpoint
CREATE TYPE "public"."rss_fetch_network_path_enum" AS ENUM('CN_ONLY', 'NON_CN_ONLY', 'CN_THEN_NON_CN');--> statement-breakpoint
CREATE TYPE "public"."rss_fetch_source_kind_enum" AS ENUM('LOCAL', 'CLOUDFLARE', 'CLOUDFLARE_FALLBACK');--> statement-breakpoint
CREATE TYPE "public"."run_record_status_enum" AS ENUM('SUCCEEDED', 'FAILED', 'SKIPPED');--> statement-breakpoint
CREATE TYPE "public"."schedule_mode_enum" AS ENUM('CRON', 'INTERVAL');--> statement-breakpoint
CREATE TYPE "public"."site_access_event_type_enum" AS ENUM('OUTBOUND_CLICK', 'EMBED_PAGEVIEW');--> statement-breakpoint
CREATE TYPE "public"."site_access_scope_enum" AS ENUM('CN_ONLY', 'NON_CN_ONLY', 'ALL');--> statement-breakpoint
CREATE TYPE "public"."site_audit_action_enum" AS ENUM('CREATE', 'UPDATE', 'DELETE', 'RESTORE');--> statement-breakpoint
CREATE TYPE "public"."site_check_mode_enum" AS ENUM('STANDARD', 'GLOBAL_FORCED');--> statement-breakpoint
CREATE TYPE "public"."site_check_region_enum" AS ENUM('CN', 'GLOBAL', 'UNKNOWN');--> statement-breakpoint
CREATE TYPE "public"."site_check_result_enum" AS ENUM('SUCCESS', 'FAILURE', 'TIMEOUT', 'DNS_ERROR', 'SSL_ERROR', 'HTTP_ERROR', 'BLOCKED');--> statement-breakpoint
CREATE TYPE "public"."site_claim_status_enum" AS ENUM('PENDING_VERIFICATION', 'PENDING_REVIEW', 'APPROVED', 'REJECTED', 'CANCELED');--> statement-breakpoint
CREATE TYPE "public"."site_claim_type_enum" AS ENUM('OWNER', 'ADMIN');--> statement-breakpoint
CREATE TYPE "public"."site_classification_status_enum" AS ENUM('COMPLETE', 'NEEDS_REVIEW');--> statement-breakpoint
CREATE TYPE "public"."site_feedback_reason_enum" AS ENUM('SITE_INFO_ERROR', 'ACCESS_ISSUE', 'FEED_ISSUE', 'CONTENT_RISK', 'COPYRIGHT', 'SPAM', 'OTHER');--> statement-breakpoint
CREATE TYPE "public"."site_status_type_enum" AS ENUM('OK', 'WARNING', 'ERROR');--> statement-breakpoint
CREATE TYPE "public"."site_tech_stack_category_enum" AS ENUM('FRAMEWORK', 'LANGUAGE');--> statement-breakpoint
CREATE TYPE "public"."site_verify_result_enum" AS ENUM('NOT_REQUESTED', 'PASSED', 'FAILED');--> statement-breakpoint
CREATE TYPE "public"."site_warning_tag_source_enum" AS ENUM('ARTICLE_FEEDBACK', 'SITE_FEEDBACK', 'MANUAL');--> statement-breakpoint
CREATE TYPE "public"."tag_type_enum" AS ENUM('MAIN', 'SUB', 'WARNING');--> statement-breakpoint
CREATE TYPE "public"."task_type_enum" AS ENUM('UPSTREAM_SYNC', 'SITE_CHECK', 'RSS_FETCH');--> statement-breakpoint
CREATE TYPE "public"."technology_type_enum" AS ENUM('FRAMEWORK', 'LANGUAGE');--> statement-breakpoint
CREATE TYPE "public"."user_oauth_provider_enum" AS ENUM('GITHUB');--> statement-breakpoint
CREATE TYPE "public"."user_role_enum" AS ENUM('SYS_ADMIN', 'ADMIN', 'USER');--> statement-breakpoint
CREATE TABLE "announcements" (
	"id" uuid PRIMARY KEY NOT NULL,
	"title" varchar(256) NOT NULL,
	"content" text,
	"status" "announcement_status_enum" DEFAULT 'DRAFT' NOT NULL,
	"publish_time" timestamp (6) with time zone,
	"expire_time" timestamp (6) with time zone,
	"expired_time" timestamp (6) with time zone,
	"created_by" uuid,
	"updated_by" uuid,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "announcements_title_not_blank_check" CHECK (btrim("announcements"."title") <> ''),
	CONSTRAINT "announcements_publish_time_required_check" CHECK ("announcements"."status" not in ('SCHEDULED', 'PUBLISHED') or "announcements"."publish_time" is not null),
	CONSTRAINT "announcements_publish_expire_timeline_check" CHECK ("announcements"."expire_time" is null or "announcements"."publish_time" is null or "announcements"."expire_time" >= "announcements"."publish_time")
);
--> statement-breakpoint
CREATE TABLE "article_feedback_audits" (
	"id" uuid PRIMARY KEY NOT NULL,
	"article_id" uuid NOT NULL,
	"site_id" uuid NOT NULL,
	"action" "article_feedback_action_enum" NOT NULL,
	"reason_type" "article_feedback_reason_enum" DEFAULT 'OTHER' NOT NULL,
	"status" "audit_status_enum" DEFAULT 'PENDING' NOT NULL,
	"feedback_content" text NOT NULL,
	"reporter_name" varchar(64),
	"reporter_email" varchar(128),
	"has_attachment" boolean DEFAULT false NOT NULL,
	"reviewer_comment" text,
	"reviewed_by" uuid,
	"reviewed_time" timestamp (6) with time zone,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "article_feedback_audits_reporter_name_not_blank_check" CHECK (btrim("article_feedback_audits"."reporter_name") <> ''),
	CONSTRAINT "article_feedback_audits_reporter_email_not_blank_check" CHECK (btrim("article_feedback_audits"."reporter_email") <> '')
);
--> statement-breakpoint
CREATE TABLE "site_audits" (
	"id" uuid PRIMARY KEY NOT NULL,
	"site_id" uuid,
	"action" "site_audit_action_enum" NOT NULL,
	"status" "audit_status_enum" DEFAULT 'PENDING' NOT NULL,
	"current_snapshot" jsonb,
	"proposed_snapshot" jsonb,
	"diff" jsonb DEFAULT '[]'::jsonb NOT NULL,
	"review_override_snapshot" jsonb,
	"review_override_diff" jsonb,
	"submit_reason" text,
	"reviewer_comment" text,
	"reviewed_by" uuid,
	"submitter_name" varchar(64),
	"submitter_email" varchar(128),
	"notify_by_email" boolean DEFAULT false NOT NULL,
	"reviewed_time" timestamp (6) with time zone,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "site_audits_submitter_name_not_blank_check" CHECK (btrim("site_audits"."submitter_name") <> ''),
	CONSTRAINT "site_audits_submitter_email_not_blank_check" CHECK (btrim("site_audits"."submitter_email") <> '')
);
--> statement-breakpoint
CREATE TABLE "site_feedback_audits" (
	"id" uuid PRIMARY KEY NOT NULL,
	"site_id" uuid NOT NULL,
	"reason_type" "site_feedback_reason_enum" DEFAULT 'OTHER' NOT NULL,
	"status" "audit_status_enum" DEFAULT 'PENDING' NOT NULL,
	"feedback_content" text NOT NULL,
	"reporter_name" varchar(64),
	"reporter_email" varchar(128),
	"notify_by_email" boolean DEFAULT false NOT NULL,
	"reviewer_comment" text,
	"reviewed_by" uuid,
	"reviewed_time" timestamp (6) with time zone,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "site_feedback_audits_reporter_name_not_blank_check" CHECK (btrim("site_feedback_audits"."reporter_name") <> ''),
	CONSTRAINT "site_feedback_audits_reporter_email_not_blank_check" CHECK (btrim("site_feedback_audits"."reporter_email") <> '')
);
--> statement-breakpoint
CREATE TABLE "tag_definitions" (
	"id" uuid PRIMARY KEY NOT NULL,
	"name" varchar(64) NOT NULL,
	"machine_key" varchar(64),
	"tag_type" "tag_type_enum" NOT NULL,
	"description" varchar(512),
	"is_enabled" boolean DEFAULT true NOT NULL,
	"merged_into_tag_id" uuid,
	"merged_by" uuid,
	"merged_time" timestamp (6) with time zone,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "tag_definitions_name_not_blank_check" CHECK (btrim("tag_definitions"."name") <> ''),
	CONSTRAINT "tag_definitions_machine_key_not_blank_check" CHECK ("tag_definitions"."machine_key" is null or btrim("tag_definitions"."machine_key") <> ''),
	CONSTRAINT "tag_definitions_warning_machine_key_required_check" CHECK ("tag_definitions"."tag_type" <> 'WARNING' or btrim(coalesce("tag_definitions"."machine_key", '')) <> ''),
	CONSTRAINT "tag_definitions_merged_into_tag_id_self_check" CHECK ("tag_definitions"."merged_into_tag_id" is null or "tag_definitions"."merged_into_tag_id" <> "tag_definitions"."id")
);
--> statement-breakpoint
CREATE TABLE "technology_catalogs" (
	"id" uuid PRIMARY KEY NOT NULL,
	"name" varchar(128) NOT NULL,
	"name_normalized" varchar(128) NOT NULL,
	"technology_type" "technology_type_enum" NOT NULL,
	"description" varchar(512),
	"official_url" varchar(256),
	"is_enabled" boolean DEFAULT true NOT NULL,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "deployments" (
	"id" uuid PRIMARY KEY NOT NULL,
	"trigger_event" varchar(64) NOT NULL,
	"status" "deployment_status_enum" DEFAULT 'PENDING' NOT NULL,
	"modules" "deployment_module_enum"[] NOT NULL,
	"delivery_id" varchar(128),
	"workflow_run_id" varchar(128),
	"workflow_run_url" varchar(256),
	"commit_sha" varchar(64),
	"git_ref" varchar(256),
	"metadata" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"raw_payload" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"started_time" timestamp (6) with time zone,
	"finished_time" timestamp (6) with time zone,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "deployments_trigger_event_not_blank_check" CHECK (btrim("deployments"."trigger_event") <> '')
);
--> statement-breakpoint
CREATE TABLE "feed_articles" (
	"id" uuid PRIMARY KEY NOT NULL,
	"site_id" uuid NOT NULL,
	"guid" varchar(512),
	"article_url" varchar(512) NOT NULL,
	"title" varchar(512) NOT NULL,
	"summary" text,
	"feed_type" "feed_type_enum",
	"source" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"published_time" timestamp (6) with time zone,
	"fetched_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"visibility" "article_visibility_enum" DEFAULT 'VISIBLE' NOT NULL,
	"visibility_reason" text,
	CONSTRAINT "feed_articles_visibility_reason_check" CHECK (("feed_articles"."visibility" = 'VISIBLE' and "feed_articles"."visibility_reason" is null) or ("feed_articles"."visibility" <> 'VISIBLE'))
);
--> statement-breakpoint
CREATE TABLE "request_configs" (
	"id" uuid PRIMARY KEY NOT NULL,
	"name" varchar(128) NOT NULL,
	"task_type" "task_type_enum" NOT NULL,
	"user_agent" varchar(512) NOT NULL,
	"timeout_ms" integer DEFAULT 20000 NOT NULL,
	"retry_max" integer DEFAULT 2 NOT NULL,
	"retry_strategy" "request_retry_strategy_enum" NOT NULL,
	"retry_base_delay_ms" integer DEFAULT 1000 NOT NULL,
	"retry_max_delay_ms" integer DEFAULT 10000 NOT NULL,
	"backoff_factor" integer DEFAULT 2 NOT NULL,
	"jitter_ratio" integer DEFAULT 0 NOT NULL,
	"wait_between_requests_ms" integer DEFAULT 0 NOT NULL,
	"default_headers" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"follow_redirects" boolean DEFAULT true NOT NULL,
	"is_enabled" boolean DEFAULT true NOT NULL,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "request_configs_name_not_blank_check" CHECK (btrim("request_configs"."name") <> ''),
	CONSTRAINT "request_configs_user_agent_not_blank_check" CHECK (btrim("request_configs"."user_agent") <> ''),
	CONSTRAINT "request_configs_timeout_positive_check" CHECK ("request_configs"."timeout_ms" > 0),
	CONSTRAINT "request_configs_retry_max_non_negative_check" CHECK ("request_configs"."retry_max" >= 0),
	CONSTRAINT "request_configs_retry_base_delay_non_negative_check" CHECK ("request_configs"."retry_base_delay_ms" >= 0),
	CONSTRAINT "request_configs_retry_max_delay_non_negative_check" CHECK ("request_configs"."retry_max_delay_ms" >= 0),
	CONSTRAINT "request_configs_backoff_factor_positive_check" CHECK ("request_configs"."backoff_factor" > 0),
	CONSTRAINT "request_configs_jitter_ratio_non_negative_check" CHECK ("request_configs"."jitter_ratio" >= 0),
	CONSTRAINT "request_configs_wait_between_requests_non_negative_check" CHECK ("request_configs"."wait_between_requests_ms" >= 0)
);
--> statement-breakpoint
CREATE TABLE "rss_fetch_runs" (
	"id" uuid PRIMARY KEY NOT NULL,
	"job_id" uuid NOT NULL,
	"site_id" uuid NOT NULL,
	"request_config_id" uuid,
	"status" "run_record_status_enum" NOT NULL,
	"effective_access_scope" "site_access_scope_enum" NOT NULL,
	"feed_url" varchar(512),
	"feed_format" "rss_feed_format_enum" DEFAULT 'UNKNOWN' NOT NULL,
	"source_kind" "rss_fetch_source_kind_enum",
	"network_path" "rss_fetch_network_path_enum" NOT NULL,
	"fallback_used" boolean DEFAULT false NOT NULL,
	"article_count" integer DEFAULT 0 NOT NULL,
	"upserted_count" integer DEFAULT 0 NOT NULL,
	"skipped_count" integer DEFAULT 0 NOT NULL,
	"error_code" varchar(64),
	"error_message" text,
	"summary_payload" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"started_time" timestamp (6) with time zone NOT NULL,
	"finished_time" timestamp (6) with time zone,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "rss_fetch_runs_article_count_non_negative_check" CHECK ("rss_fetch_runs"."article_count" >= 0),
	CONSTRAINT "rss_fetch_runs_upserted_count_non_negative_check" CHECK ("rss_fetch_runs"."upserted_count" >= 0),
	CONSTRAINT "rss_fetch_runs_skipped_count_non_negative_check" CHECK ("rss_fetch_runs"."skipped_count" >= 0)
);
--> statement-breakpoint
CREATE TABLE "site_check_runs" (
	"id" uuid PRIMARY KEY NOT NULL,
	"job_id" uuid NOT NULL,
	"site_id" uuid NOT NULL,
	"request_config_id" uuid,
	"status" "run_record_status_enum" NOT NULL,
	"availability_result" "site_check_result_enum" NOT NULL,
	"verify_result" "site_verify_result_enum" DEFAULT 'NOT_REQUESTED' NOT NULL,
	"effective_access_scope" "site_access_scope_enum" NOT NULL,
	"derived_access_scope" "site_access_scope_enum" NOT NULL,
	"derived_status" "site_status_type_enum" NOT NULL,
	"check_mode" "site_check_mode_enum" DEFAULT 'STANDARD' NOT NULL,
	"content_validation_status" "content_validation_status_enum" DEFAULT 'NOT_REQUESTED' NOT NULL,
	"content_validation_payload" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"probe_summary" jsonb DEFAULT '[]'::jsonb NOT NULL,
	"response_time_ms" integer,
	"duration_ms" integer,
	"jitter_ms" integer,
	"final_url" varchar(512),
	"error_code" varchar(64),
	"error_message" text,
	"started_time" timestamp (6) with time zone NOT NULL,
	"finished_time" timestamp (6) with time zone,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "site_check_runs_response_time_non_negative_check" CHECK ("site_check_runs"."response_time_ms" is null or "site_check_runs"."response_time_ms" >= 0),
	CONSTRAINT "site_check_runs_duration_non_negative_check" CHECK ("site_check_runs"."duration_ms" is null or "site_check_runs"."duration_ms" >= 0),
	CONSTRAINT "site_check_runs_jitter_non_negative_check" CHECK ("site_check_runs"."jitter_ms" is null or "site_check_runs"."jitter_ms" >= 0)
);
--> statement-breakpoint
CREATE TABLE "site_claims" (
	"id" uuid PRIMARY KEY NOT NULL,
	"site_id" uuid NOT NULL,
	"user_id" uuid NOT NULL,
	"claim_type" "site_claim_type_enum" DEFAULT 'OWNER' NOT NULL,
	"status" "site_claim_status_enum" DEFAULT 'PENDING_VERIFICATION' NOT NULL,
	"verification_token" varchar(128),
	"verification_expires_time" timestamp (6) with time zone,
	"verification_note" text,
	"review_note" text,
	"reviewed_by" uuid,
	"submitted_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"reviewed_time" timestamp (6) with time zone,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "site_warning_tags" (
	"id" uuid PRIMARY KEY NOT NULL,
	"site_id" uuid NOT NULL,
	"tag_id" uuid NOT NULL,
	"source" "site_warning_tag_source_enum" NOT NULL,
	"source_site_audit_id" uuid,
	"source_article_feedback_audit_id" uuid,
	"note" text,
	"created_by" uuid,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "program_technology_stacks" (
	"id" uuid PRIMARY KEY NOT NULL,
	"program_id" uuid NOT NULL,
	"catalog_id" uuid,
	"category" "site_tech_stack_category_enum" NOT NULL,
	"name_custom" varchar(128),
	"name_normalized" varchar(128) NOT NULL,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "program_technology_stacks_name_normalized_not_blank_check" CHECK (btrim("program_technology_stacks"."name_normalized") <> '')
);
--> statement-breakpoint
CREATE TABLE "programs" (
	"id" uuid PRIMARY KEY NOT NULL,
	"name" varchar(128) NOT NULL,
	"name_normalized" varchar(128) NOT NULL,
	"is_open_source" boolean DEFAULT false NOT NULL,
	"website_url" varchar(512),
	"repo_url" varchar(512),
	"is_enabled" boolean DEFAULT true NOT NULL,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "programs_name_not_blank_check" CHECK (btrim("programs"."name") <> ''),
	CONSTRAINT "programs_name_normalized_not_blank_check" CHECK (btrim("programs"."name_normalized") <> '')
);
--> statement-breakpoint
CREATE TABLE "site_access_events" (
	"id" uuid PRIMARY KEY NOT NULL,
	"site_id" uuid NOT NULL,
	"event_type" "site_access_event_type_enum" DEFAULT 'OUTBOUND_CLICK' NOT NULL,
	"source" varchar(64) DEFAULT 'UNKNOWN' NOT NULL,
	"referer_host" varchar(256),
	"path" varchar(512),
	"user_agent" varchar(512),
	"occurred_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "site_access_events_source_not_blank_check" CHECK (btrim("site_access_events"."source") <> '')
);
--> statement-breakpoint
CREATE TABLE "site_architectures" (
	"site_id" uuid PRIMARY KEY NOT NULL,
	"program_id" uuid NOT NULL,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "site_tags" (
	"site_id" uuid NOT NULL,
	"tag_id" uuid NOT NULL,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "site_tags_pkey" PRIMARY KEY("site_id","tag_id")
);
--> statement-breakpoint
CREATE TABLE "sites" (
	"id" uuid PRIMARY KEY NOT NULL,
	"bid" varchar(64),
	"name" varchar(64) NOT NULL,
	"url" varchar(256) NOT NULL,
	"sign" text DEFAULT '',
	"icon_base64" text,
	"feed" jsonb DEFAULT '[]'::jsonb NOT NULL,
	"from" "from_source_enum"[] DEFAULT '{}' NOT NULL,
	"classification_status" "site_classification_status_enum" DEFAULT 'COMPLETE' NOT NULL,
	"sitemap" varchar(256),
	"link_page" varchar(256),
	"join_time" timestamp (6) with time zone NOT NULL,
	"update_time" timestamp (6) with time zone NOT NULL,
	"access_scope" "site_access_scope_enum" DEFAULT 'ALL' NOT NULL,
	"status" "site_status_type_enum" DEFAULT 'OK' NOT NULL,
	"is_show" boolean DEFAULT true NOT NULL,
	"recommend" boolean DEFAULT false NOT NULL,
	"reason" text,
	CONSTRAINT "sites_bid_unique" UNIQUE("bid"),
	CONSTRAINT "sites_url_unique" UNIQUE("url"),
	CONSTRAINT "sites_bid_not_blank_check" CHECK (btrim("sites"."bid") <> ''),
	CONSTRAINT "sites_name_not_blank_check" CHECK (btrim("sites"."name") <> '')
);
--> statement-breakpoint
CREATE TABLE "jobs" (
	"id" uuid PRIMARY KEY NOT NULL,
	"schedule_id" uuid,
	"task_type" "task_type_enum" NOT NULL,
	"trigger_source" "job_trigger_source_enum" NOT NULL,
	"status" "job_status_enum" DEFAULT 'PENDING' NOT NULL,
	"payload" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"result" jsonb,
	"retry_root_job_id" uuid,
	"retry_parent_job_id" uuid,
	"retry_sequence" integer DEFAULT 0 NOT NULL,
	"run_at" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"locked_at" timestamp (6) with time zone,
	"locked_by" varchar(128),
	"heartbeat_time" timestamp (6) with time zone,
	"started_time" timestamp (6) with time zone,
	"finished_time" timestamp (6) with time zone,
	"error_code" varchar(64),
	"error_message" text,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "jobs_retry_sequence_non_negative_check" CHECK ("jobs"."retry_sequence" >= 0)
);
--> statement-breakpoint
CREATE TABLE "task_schedules" (
	"id" uuid PRIMARY KEY NOT NULL,
	"name" varchar(128) NOT NULL,
	"task_type" "task_type_enum" NOT NULL,
	"schedule_mode" "schedule_mode_enum" NOT NULL,
	"request_config_id" uuid,
	"is_enabled" boolean DEFAULT true NOT NULL,
	"schedule_config" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"payload_template" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"next_run_time" timestamp (6) with time zone,
	"last_run_time" timestamp (6) with time zone,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "task_schedules_name_not_blank_check" CHECK (btrim("task_schedules"."name") <> '')
);
--> statement-breakpoint
CREATE TABLE "upstream_site_bindings" (
	"id" uuid PRIMARY KEY NOT NULL,
	"source_id" uuid NOT NULL,
	"site_id" uuid NOT NULL,
	"external_key" varchar(256) NOT NULL,
	"external_url" varchar(512),
	"last_seen_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "upstream_site_bindings_external_key_not_blank_check" CHECK (btrim("upstream_site_bindings"."external_key") <> '')
);
--> statement-breakpoint
CREATE TABLE "upstream_sources" (
	"id" uuid PRIMARY KEY NOT NULL,
	"source_key" varchar(64) NOT NULL,
	"label" varchar(128) NOT NULL,
	"base_url" varchar(512) NOT NULL,
	"adapter_key" varchar(64) NOT NULL,
	"request_config_id" uuid,
	"is_enabled" boolean DEFAULT true NOT NULL,
	"remark" varchar(512),
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "upstream_sources_source_key_unique" UNIQUE("source_key"),
	CONSTRAINT "upstream_sources_source_key_not_blank_check" CHECK (btrim("upstream_sources"."source_key") <> ''),
	CONSTRAINT "upstream_sources_label_not_blank_check" CHECK (btrim("upstream_sources"."label") <> ''),
	CONSTRAINT "upstream_sources_base_url_not_blank_check" CHECK (btrim("upstream_sources"."base_url") <> ''),
	CONSTRAINT "upstream_sources_adapter_key_not_blank_check" CHECK (btrim("upstream_sources"."adapter_key") <> '')
);
--> statement-breakpoint
CREATE TABLE "upstream_sync_runs" (
	"id" uuid PRIMARY KEY NOT NULL,
	"job_id" uuid NOT NULL,
	"source_id" uuid NOT NULL,
	"request_config_id" uuid,
	"new_site_count" integer DEFAULT 0 NOT NULL,
	"updated_site_count" integer DEFAULT 0 NOT NULL,
	"skipped_count" integer DEFAULT 0 NOT NULL,
	"duration_ms" integer,
	"error_code" varchar(64),
	"error_message" text,
	"summary_payload" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"started_time" timestamp (6) with time zone NOT NULL,
	"finished_time" timestamp (6) with time zone,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "upstream_sync_runs_new_site_count_non_negative_check" CHECK ("upstream_sync_runs"."new_site_count" >= 0),
	CONSTRAINT "upstream_sync_runs_updated_site_count_non_negative_check" CHECK ("upstream_sync_runs"."updated_site_count" >= 0),
	CONSTRAINT "upstream_sync_runs_skipped_count_non_negative_check" CHECK ("upstream_sync_runs"."skipped_count" >= 0),
	CONSTRAINT "upstream_sync_runs_duration_non_negative_check" CHECK ("upstream_sync_runs"."duration_ms" is null or "upstream_sync_runs"."duration_ms" >= 0)
);
--> statement-breakpoint
CREATE TABLE "user_api_tokens" (
	"id" uuid PRIMARY KEY NOT NULL,
	"user_id" uuid NOT NULL,
	"name" varchar(64) NOT NULL,
	"token_hash" varchar(256) NOT NULL,
	"scopes" jsonb DEFAULT '[]'::jsonb NOT NULL,
	"is_active" boolean DEFAULT true NOT NULL,
	"last_used_time" timestamp (6) with time zone,
	"expires_time" timestamp (6) with time zone,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "user_api_tokens_token_hash_unique" UNIQUE("token_hash")
);
--> statement-breakpoint
CREATE TABLE "user_email_verification_tokens" (
	"id" uuid PRIMARY KEY NOT NULL,
	"user_id" uuid NOT NULL,
	"email" varchar(128) NOT NULL,
	"token_hash" varchar(128) NOT NULL,
	"expires_time" timestamp (6) with time zone NOT NULL,
	"consumed_time" timestamp (6) with time zone,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "user_management_permissions" (
	"id" uuid PRIMARY KEY NOT NULL,
	"user_id" uuid NOT NULL,
	"permission_key" "management_permission_enum" NOT NULL,
	"granted_by" uuid,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "user_oauth_accounts" (
	"id" uuid PRIMARY KEY NOT NULL,
	"user_id" uuid NOT NULL,
	"provider" "user_oauth_provider_enum" NOT NULL,
	"provider_user_id" varchar(128) NOT NULL,
	"provider_username" varchar(128),
	"access_token" text,
	"refresh_token" text,
	"expires_time" timestamp (6) with time zone,
	"scopes" jsonb DEFAULT '[]'::jsonb NOT NULL,
	"profile" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "user_password_reset_tokens" (
	"id" uuid PRIMARY KEY NOT NULL,
	"user_id" uuid NOT NULL,
	"email" varchar(128) NOT NULL,
	"token_hash" varchar(128) NOT NULL,
	"expires_time" timestamp (6) with time zone NOT NULL,
	"consumed_time" timestamp (6) with time zone,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "user_sites" (
	"id" uuid PRIMARY KEY NOT NULL,
	"user_id" uuid NOT NULL,
	"site_id" uuid NOT NULL,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "users" (
	"id" uuid PRIMARY KEY NOT NULL,
	"username" varchar(64) NOT NULL,
	"nickname" varchar(64) NOT NULL,
	"avatar_url" varchar(256),
	"email" varchar(128) NOT NULL,
	"password_hash" varchar(256),
	"role" "user_role_enum" DEFAULT 'USER' NOT NULL,
	"is_active" boolean DEFAULT true NOT NULL,
	"is_verified" boolean DEFAULT false NOT NULL,
	"profile" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"settings" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"metadata" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"last_login_time" timestamp (6) with time zone,
	"created_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	"updated_time" timestamp (6) with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "users_username_not_blank_check" CHECK (btrim("users"."username") <> ''),
	CONSTRAINT "users_nickname_not_blank_check" CHECK (btrim("users"."nickname") <> '')
);
--> statement-breakpoint
ALTER TABLE "announcements" ADD CONSTRAINT "announcements_created_by_users_id_fk" FOREIGN KEY ("created_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "announcements" ADD CONSTRAINT "announcements_updated_by_users_id_fk" FOREIGN KEY ("updated_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "article_feedback_audits" ADD CONSTRAINT "article_feedback_audits_article_id_feed_articles_id_fk" FOREIGN KEY ("article_id") REFERENCES "public"."feed_articles"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "article_feedback_audits" ADD CONSTRAINT "article_feedback_audits_site_id_sites_id_fk" FOREIGN KEY ("site_id") REFERENCES "public"."sites"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "article_feedback_audits" ADD CONSTRAINT "article_feedback_audits_reviewed_by_users_id_fk" FOREIGN KEY ("reviewed_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_audits" ADD CONSTRAINT "site_audits_site_id_sites_id_fk" FOREIGN KEY ("site_id") REFERENCES "public"."sites"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_audits" ADD CONSTRAINT "site_audits_reviewed_by_users_id_fk" FOREIGN KEY ("reviewed_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_feedback_audits" ADD CONSTRAINT "site_feedback_audits_site_id_sites_id_fk" FOREIGN KEY ("site_id") REFERENCES "public"."sites"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_feedback_audits" ADD CONSTRAINT "site_feedback_audits_reviewed_by_users_id_fk" FOREIGN KEY ("reviewed_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "tag_definitions" ADD CONSTRAINT "tag_definitions_merged_into_tag_id_tag_definitions_id_fk" FOREIGN KEY ("merged_into_tag_id") REFERENCES "public"."tag_definitions"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "tag_definitions" ADD CONSTRAINT "tag_definitions_merged_by_users_id_fk" FOREIGN KEY ("merged_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "feed_articles" ADD CONSTRAINT "feed_articles_site_id_sites_id_fk" FOREIGN KEY ("site_id") REFERENCES "public"."sites"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "rss_fetch_runs" ADD CONSTRAINT "rss_fetch_runs_job_id_jobs_id_fk" FOREIGN KEY ("job_id") REFERENCES "public"."jobs"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "rss_fetch_runs" ADD CONSTRAINT "rss_fetch_runs_site_id_sites_id_fk" FOREIGN KEY ("site_id") REFERENCES "public"."sites"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "rss_fetch_runs" ADD CONSTRAINT "rss_fetch_runs_request_config_id_request_configs_id_fk" FOREIGN KEY ("request_config_id") REFERENCES "public"."request_configs"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_check_runs" ADD CONSTRAINT "site_check_runs_job_id_jobs_id_fk" FOREIGN KEY ("job_id") REFERENCES "public"."jobs"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_check_runs" ADD CONSTRAINT "site_check_runs_site_id_sites_id_fk" FOREIGN KEY ("site_id") REFERENCES "public"."sites"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_check_runs" ADD CONSTRAINT "site_check_runs_request_config_id_request_configs_id_fk" FOREIGN KEY ("request_config_id") REFERENCES "public"."request_configs"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_claims" ADD CONSTRAINT "site_claims_site_id_sites_id_fk" FOREIGN KEY ("site_id") REFERENCES "public"."sites"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_claims" ADD CONSTRAINT "site_claims_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_claims" ADD CONSTRAINT "site_claims_reviewed_by_users_id_fk" FOREIGN KEY ("reviewed_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_warning_tags" ADD CONSTRAINT "site_warning_tags_site_id_sites_id_fk" FOREIGN KEY ("site_id") REFERENCES "public"."sites"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_warning_tags" ADD CONSTRAINT "site_warning_tags_tag_id_tag_definitions_id_fk" FOREIGN KEY ("tag_id") REFERENCES "public"."tag_definitions"("id") ON DELETE restrict ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_warning_tags" ADD CONSTRAINT "site_warning_tags_source_site_audit_id_site_audits_id_fk" FOREIGN KEY ("source_site_audit_id") REFERENCES "public"."site_audits"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_warning_tags" ADD CONSTRAINT "site_warning_tags_source_article_feedback_audit_id_article_feedback_audits_id_fk" FOREIGN KEY ("source_article_feedback_audit_id") REFERENCES "public"."article_feedback_audits"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_warning_tags" ADD CONSTRAINT "site_warning_tags_created_by_users_id_fk" FOREIGN KEY ("created_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "program_technology_stacks" ADD CONSTRAINT "program_technology_stacks_program_id_programs_id_fk" FOREIGN KEY ("program_id") REFERENCES "public"."programs"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "program_technology_stacks" ADD CONSTRAINT "program_technology_stacks_catalog_id_technology_catalogs_id_fk" FOREIGN KEY ("catalog_id") REFERENCES "public"."technology_catalogs"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_access_events" ADD CONSTRAINT "site_access_events_site_id_sites_id_fk" FOREIGN KEY ("site_id") REFERENCES "public"."sites"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_architectures" ADD CONSTRAINT "site_architectures_site_id_sites_id_fk" FOREIGN KEY ("site_id") REFERENCES "public"."sites"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_architectures" ADD CONSTRAINT "site_architectures_program_id_programs_id_fk" FOREIGN KEY ("program_id") REFERENCES "public"."programs"("id") ON DELETE restrict ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_tags" ADD CONSTRAINT "site_tags_site_id_sites_id_fk" FOREIGN KEY ("site_id") REFERENCES "public"."sites"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "site_tags" ADD CONSTRAINT "site_tags_tag_id_tag_definitions_id_fk" FOREIGN KEY ("tag_id") REFERENCES "public"."tag_definitions"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "jobs" ADD CONSTRAINT "jobs_schedule_id_task_schedules_id_fk" FOREIGN KEY ("schedule_id") REFERENCES "public"."task_schedules"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "task_schedules" ADD CONSTRAINT "task_schedules_request_config_id_request_configs_id_fk" FOREIGN KEY ("request_config_id") REFERENCES "public"."request_configs"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "upstream_site_bindings" ADD CONSTRAINT "upstream_site_bindings_source_id_upstream_sources_id_fk" FOREIGN KEY ("source_id") REFERENCES "public"."upstream_sources"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "upstream_site_bindings" ADD CONSTRAINT "upstream_site_bindings_site_id_sites_id_fk" FOREIGN KEY ("site_id") REFERENCES "public"."sites"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "upstream_sources" ADD CONSTRAINT "upstream_sources_request_config_id_request_configs_id_fk" FOREIGN KEY ("request_config_id") REFERENCES "public"."request_configs"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "upstream_sync_runs" ADD CONSTRAINT "upstream_sync_runs_job_id_jobs_id_fk" FOREIGN KEY ("job_id") REFERENCES "public"."jobs"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "upstream_sync_runs" ADD CONSTRAINT "upstream_sync_runs_source_id_upstream_sources_id_fk" FOREIGN KEY ("source_id") REFERENCES "public"."upstream_sources"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "upstream_sync_runs" ADD CONSTRAINT "upstream_sync_runs_request_config_id_request_configs_id_fk" FOREIGN KEY ("request_config_id") REFERENCES "public"."request_configs"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "user_api_tokens" ADD CONSTRAINT "user_api_tokens_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "user_email_verification_tokens" ADD CONSTRAINT "user_email_verification_tokens_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "user_management_permissions" ADD CONSTRAINT "user_management_permissions_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "user_management_permissions" ADD CONSTRAINT "user_management_permissions_granted_by_users_id_fk" FOREIGN KEY ("granted_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "user_oauth_accounts" ADD CONSTRAINT "user_oauth_accounts_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "user_password_reset_tokens" ADD CONSTRAINT "user_password_reset_tokens_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "user_sites" ADD CONSTRAINT "user_sites_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
ALTER TABLE "user_sites" ADD CONSTRAINT "user_sites_site_id_sites_id_fk" FOREIGN KEY ("site_id") REFERENCES "public"."sites"("id") ON DELETE cascade ON UPDATE cascade;--> statement-breakpoint
CREATE INDEX "announcements_status_publish_time_index" ON "announcements" USING btree ("status","publish_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "announcements_expire_time_index" ON "announcements" USING btree ("expire_time");--> statement-breakpoint
CREATE INDEX "announcements_created_time_index" ON "announcements" USING btree ("created_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "article_feedback_audits_article_id_created_time_index" ON "article_feedback_audits" USING btree ("article_id","created_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "article_feedback_audits_site_id_status_index" ON "article_feedback_audits" USING btree ("site_id","status");--> statement-breakpoint
CREATE INDEX "article_feedback_audits_status_created_time_index" ON "article_feedback_audits" USING btree ("status","created_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "article_feedback_audits_action_status_index" ON "article_feedback_audits" USING btree ("action","status");--> statement-breakpoint
CREATE INDEX "article_feedback_audits_reviewed_by_index" ON "article_feedback_audits" USING btree ("reviewed_by");--> statement-breakpoint
CREATE INDEX "site_audits_site_id_created_time_index" ON "site_audits" USING btree ("site_id","created_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "site_audits_status_created_time_index" ON "site_audits" USING btree ("status","created_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "site_audits_action_status_index" ON "site_audits" USING btree ("action","status");--> statement-breakpoint
CREATE INDEX "site_audits_reviewed_by_index" ON "site_audits" USING btree ("reviewed_by");--> statement-breakpoint
CREATE INDEX "site_feedback_audits_site_id_created_time_index" ON "site_feedback_audits" USING btree ("site_id","created_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "site_feedback_audits_status_created_time_index" ON "site_feedback_audits" USING btree ("status","created_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "site_feedback_audits_reason_status_index" ON "site_feedback_audits" USING btree ("reason_type","status");--> statement-breakpoint
CREATE INDEX "site_feedback_audits_reviewed_by_index" ON "site_feedback_audits" USING btree ("reviewed_by");--> statement-breakpoint
CREATE UNIQUE INDEX "tag_definitions_name_type_index" ON "tag_definitions" USING btree ("name","tag_type");--> statement-breakpoint
CREATE UNIQUE INDEX "tag_definitions_type_machine_key_index" ON "tag_definitions" USING btree ("tag_type","machine_key");--> statement-breakpoint
CREATE INDEX "tag_definitions_type_enabled_index" ON "tag_definitions" USING btree ("tag_type","is_enabled");--> statement-breakpoint
CREATE INDEX "tag_definitions_merged_into_tag_id_index" ON "tag_definitions" USING btree ("merged_into_tag_id");--> statement-breakpoint
CREATE UNIQUE INDEX "technology_catalogs_name_type_index" ON "technology_catalogs" USING btree ("name","technology_type");--> statement-breakpoint
CREATE UNIQUE INDEX "technology_catalogs_name_normalized_type_index" ON "technology_catalogs" USING btree ("name_normalized","technology_type");--> statement-breakpoint
CREATE INDEX "technology_catalogs_type_enabled_index" ON "technology_catalogs" USING btree ("technology_type","is_enabled");--> statement-breakpoint
CREATE INDEX "technology_catalogs_name_normalized_index" ON "technology_catalogs" USING btree ("name_normalized");--> statement-breakpoint
CREATE INDEX "deployments_status_created_time_index" ON "deployments" USING btree ("status","created_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "deployments_commit_sha_created_time_index" ON "deployments" USING btree ("commit_sha","created_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "deployments_trigger_event_created_time_index" ON "deployments" USING btree ("trigger_event","created_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "deployments_started_time_index" ON "deployments" USING btree ("started_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "deployments_workflow_run_id_index" ON "deployments" USING btree ("workflow_run_id");--> statement-breakpoint
CREATE INDEX "deployments_delivery_id_index" ON "deployments" USING btree ("delivery_id");--> statement-breakpoint
CREATE INDEX "deployments_modules_gin_index" ON "deployments" USING gin ("modules");--> statement-breakpoint
CREATE UNIQUE INDEX "feed_articles_site_id_guid_index" ON "feed_articles" USING btree ("site_id","guid") WHERE "feed_articles"."guid" is not null and "feed_articles"."guid" <> '';--> statement-breakpoint
CREATE UNIQUE INDEX "feed_articles_site_id_url_index" ON "feed_articles" USING btree ("site_id","article_url");--> statement-breakpoint
CREATE INDEX "feed_articles_site_id_published_time_index" ON "feed_articles" USING btree ("site_id","published_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "feed_articles_site_id_visibility_published_time_index" ON "feed_articles" USING btree ("site_id","visibility","published_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "feed_articles_feed_type_index" ON "feed_articles" USING btree ("feed_type");--> statement-breakpoint
CREATE INDEX "feed_articles_visibility_published_time_index" ON "feed_articles" USING btree ("visibility","published_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "feed_articles_fetched_time_index" ON "feed_articles" USING btree ("fetched_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "rss_fetch_runs_job_id_started_time_index" ON "rss_fetch_runs" USING btree ("job_id","started_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "rss_fetch_runs_site_id_started_time_index" ON "rss_fetch_runs" USING btree ("site_id","started_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "site_check_runs_job_id_started_time_index" ON "site_check_runs" USING btree ("job_id","started_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "site_check_runs_site_id_started_time_index" ON "site_check_runs" USING btree ("site_id","started_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "site_check_runs_site_id_status_started_time_index" ON "site_check_runs" USING btree ("site_id","status","started_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "site_claims_site_id_status_index" ON "site_claims" USING btree ("site_id","status");--> statement-breakpoint
CREATE INDEX "site_claims_user_id_status_index" ON "site_claims" USING btree ("user_id","status");--> statement-breakpoint
CREATE INDEX "site_claims_claim_type_index" ON "site_claims" USING btree ("claim_type");--> statement-breakpoint
CREATE INDEX "site_claims_verification_token_index" ON "site_claims" USING btree ("verification_token");--> statement-breakpoint
CREATE INDEX "site_claims_status_submitted_time_index" ON "site_claims" USING btree ("status","submitted_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "site_claims_submitted_time_index" ON "site_claims" USING btree ("submitted_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "site_warning_tags_site_id_index" ON "site_warning_tags" USING btree ("site_id");--> statement-breakpoint
CREATE INDEX "site_warning_tags_site_id_tag_id_index" ON "site_warning_tags" USING btree ("site_id","tag_id");--> statement-breakpoint
CREATE INDEX "site_warning_tags_site_id_source_index" ON "site_warning_tags" USING btree ("site_id","source");--> statement-breakpoint
CREATE INDEX "site_warning_tags_source_site_audit_id_index" ON "site_warning_tags" USING btree ("source_site_audit_id");--> statement-breakpoint
CREATE INDEX "site_warning_tags_source_article_feedback_audit_id_index" ON "site_warning_tags" USING btree ("source_article_feedback_audit_id");--> statement-breakpoint
CREATE INDEX "site_warning_tags_tag_id_index" ON "site_warning_tags" USING btree ("tag_id");--> statement-breakpoint
CREATE INDEX "site_warning_tags_source_index" ON "site_warning_tags" USING btree ("source");--> statement-breakpoint
CREATE INDEX "site_warning_tags_created_by_index" ON "site_warning_tags" USING btree ("created_by");--> statement-breakpoint
CREATE INDEX "site_warning_tags_created_time_index" ON "site_warning_tags" USING btree ("created_time" DESC NULLS LAST);--> statement-breakpoint
CREATE UNIQUE INDEX "site_warning_tags_site_id_tag_id_source_unique" ON "site_warning_tags" USING btree ("site_id","tag_id","source");--> statement-breakpoint
CREATE INDEX "program_technology_stacks_program_id_index" ON "program_technology_stacks" USING btree ("program_id");--> statement-breakpoint
CREATE INDEX "program_technology_stacks_catalog_id_index" ON "program_technology_stacks" USING btree ("catalog_id");--> statement-breakpoint
CREATE INDEX "program_technology_stacks_category_index" ON "program_technology_stacks" USING btree ("category");--> statement-breakpoint
CREATE INDEX "program_technology_stacks_name_normalized_index" ON "program_technology_stacks" USING btree ("name_normalized");--> statement-breakpoint
CREATE UNIQUE INDEX "program_technology_stacks_program_category_name_unique" ON "program_technology_stacks" USING btree ("program_id","category","name_normalized");--> statement-breakpoint
CREATE UNIQUE INDEX "programs_name_normalized_unique" ON "programs" USING btree ("name_normalized");--> statement-breakpoint
CREATE INDEX "programs_enabled_name_index" ON "programs" USING btree ("is_enabled","name");--> statement-breakpoint
CREATE INDEX "site_access_events_site_id_occurred_time_index" ON "site_access_events" USING btree ("site_id","occurred_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "site_access_events_site_id_event_type_occurred_time_index" ON "site_access_events" USING btree ("site_id","event_type","occurred_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "site_access_events_site_id_event_type_source_occurred_time_index" ON "site_access_events" USING btree ("site_id","event_type","source","occurred_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "site_access_events_event_type_occurred_time_index" ON "site_access_events" USING btree ("event_type","occurred_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "site_access_events_referer_host_index" ON "site_access_events" USING btree ("referer_host");--> statement-breakpoint
CREATE INDEX "site_access_events_occurred_time_index" ON "site_access_events" USING btree ("occurred_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "site_architectures_program_id_index" ON "site_architectures" USING btree ("program_id");--> statement-breakpoint
CREATE INDEX "site_tags_site_id_index" ON "site_tags" USING btree ("site_id");--> statement-breakpoint
CREATE INDEX "site_tags_tag_id_index" ON "site_tags" USING btree ("tag_id");--> statement-breakpoint
CREATE INDEX "sites_from_gin_index" ON "sites" USING gin ("from");--> statement-breakpoint
CREATE INDEX "sites_name_index" ON "sites" USING btree ("name");--> statement-breakpoint
CREATE INDEX "sites_name_lower_index" ON "sites" USING btree (lower("name"));--> statement-breakpoint
CREATE INDEX "sites_access_scope_index" ON "sites" USING btree ("access_scope");--> statement-breakpoint
CREATE INDEX "sites_status_index" ON "sites" USING btree ("status");--> statement-breakpoint
CREATE INDEX "sites_is_show_index" ON "sites" USING btree ("is_show");--> statement-breakpoint
CREATE INDEX "sites_recommend_index" ON "sites" USING btree ("recommend");--> statement-breakpoint
CREATE INDEX "sites_join_time_index" ON "sites" USING btree ("join_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "sites_update_time_index" ON "sites" USING btree ("update_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "jobs_status_run_at_index" ON "jobs" USING btree ("status","run_at");--> statement-breakpoint
CREATE INDEX "jobs_schedule_created_time_index" ON "jobs" USING btree ("schedule_id","created_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "jobs_task_type_created_time_index" ON "jobs" USING btree ("task_type","created_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "jobs_locked_by_heartbeat_time_index" ON "jobs" USING btree ("locked_by","heartbeat_time");--> statement-breakpoint
CREATE INDEX "jobs_retry_root_created_time_index" ON "jobs" USING btree ("retry_root_job_id","created_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "task_schedules_task_type_enabled_index" ON "task_schedules" USING btree ("task_type","is_enabled");--> statement-breakpoint
CREATE INDEX "task_schedules_enabled_next_run_index" ON "task_schedules" USING btree ("is_enabled","next_run_time");--> statement-breakpoint
CREATE INDEX "upstream_site_bindings_source_site_index" ON "upstream_site_bindings" USING btree ("source_id","site_id");--> statement-breakpoint
CREATE INDEX "upstream_site_bindings_source_external_key_index" ON "upstream_site_bindings" USING btree ("source_id","external_key");--> statement-breakpoint
CREATE INDEX "upstream_sources_enabled_index" ON "upstream_sources" USING btree ("is_enabled");--> statement-breakpoint
CREATE INDEX "upstream_sync_runs_job_id_started_time_index" ON "upstream_sync_runs" USING btree ("job_id","started_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "upstream_sync_runs_source_id_started_time_index" ON "upstream_sync_runs" USING btree ("source_id","started_time" DESC NULLS LAST);--> statement-breakpoint
CREATE INDEX "user_api_tokens_user_id_index" ON "user_api_tokens" USING btree ("user_id");--> statement-breakpoint
CREATE INDEX "user_api_tokens_user_id_active_index" ON "user_api_tokens" USING btree ("user_id","is_active");--> statement-breakpoint
CREATE INDEX "user_api_tokens_expires_time_index" ON "user_api_tokens" USING btree ("expires_time");--> statement-breakpoint
CREATE UNIQUE INDEX "user_email_verification_tokens_token_hash_index" ON "user_email_verification_tokens" USING btree ("token_hash");--> statement-breakpoint
CREATE INDEX "user_email_verification_tokens_user_id_index" ON "user_email_verification_tokens" USING btree ("user_id");--> statement-breakpoint
CREATE INDEX "user_email_verification_tokens_email_index" ON "user_email_verification_tokens" USING btree ("email");--> statement-breakpoint
CREATE INDEX "user_email_verification_tokens_expires_time_index" ON "user_email_verification_tokens" USING btree ("expires_time");--> statement-breakpoint
CREATE UNIQUE INDEX "user_management_permissions_user_permission_index" ON "user_management_permissions" USING btree ("user_id","permission_key");--> statement-breakpoint
CREATE INDEX "user_management_permissions_user_id_index" ON "user_management_permissions" USING btree ("user_id");--> statement-breakpoint
CREATE INDEX "user_management_permissions_granted_by_index" ON "user_management_permissions" USING btree ("granted_by");--> statement-breakpoint
CREATE INDEX "user_management_permissions_permission_key_index" ON "user_management_permissions" USING btree ("permission_key");--> statement-breakpoint
CREATE UNIQUE INDEX "user_oauth_accounts_provider_user_id_index" ON "user_oauth_accounts" USING btree ("provider","provider_user_id");--> statement-breakpoint
CREATE UNIQUE INDEX "user_oauth_accounts_user_provider_index" ON "user_oauth_accounts" USING btree ("user_id","provider");--> statement-breakpoint
CREATE INDEX "user_oauth_accounts_user_id_index" ON "user_oauth_accounts" USING btree ("user_id");--> statement-breakpoint
CREATE INDEX "user_oauth_accounts_provider_index" ON "user_oauth_accounts" USING btree ("provider");--> statement-breakpoint
CREATE UNIQUE INDEX "user_password_reset_tokens_token_hash_index" ON "user_password_reset_tokens" USING btree ("token_hash");--> statement-breakpoint
CREATE INDEX "user_password_reset_tokens_user_id_index" ON "user_password_reset_tokens" USING btree ("user_id");--> statement-breakpoint
CREATE INDEX "user_password_reset_tokens_email_index" ON "user_password_reset_tokens" USING btree ("email");--> statement-breakpoint
CREATE INDEX "user_password_reset_tokens_expires_time_index" ON "user_password_reset_tokens" USING btree ("expires_time");--> statement-breakpoint
CREATE UNIQUE INDEX "user_sites_site_id_index" ON "user_sites" USING btree ("site_id");--> statement-breakpoint
CREATE INDEX "user_sites_user_id_index" ON "user_sites" USING btree ("user_id");--> statement-breakpoint
CREATE UNIQUE INDEX "users_email_lower_index" ON "users" USING btree (lower("email"));--> statement-breakpoint
CREATE UNIQUE INDEX "users_username_lower_index" ON "users" USING btree (lower("username"));--> statement-breakpoint
CREATE INDEX "users_role_active_index" ON "users" USING btree ("role","is_active");--> statement-breakpoint
CREATE INDEX "users_created_time_index" ON "users" USING btree ("created_time" DESC NULLS LAST);--> statement-breakpoint
CREATE VIEW "public"."latest_site_checks" AS (
  select distinct on (scr.site_id)
    scr.site_id as site_id,
    scr.id as check_id,
    'UNKNOWN'::varchar as region,
    scr.derived_status as result,
    null::integer as status_code,
    scr.response_time_ms as response_time_ms,
    scr.duration_ms as duration_ms,
    scr.final_url as final_url,
    (scr.verify_result = 'PASSED') as content_verified,
    scr.started_time as check_time
  from site_check_runs scr
  order by scr.site_id, scr.started_time desc, scr.id desc
);--> statement-breakpoint
CREATE VIEW "public"."public_site_directory_read_view" AS (
  select
    s.id as site_id,
    s.bid as bid,
    s.name as name,
    s.url as url,
    coalesce(s.sign, '') as sign,
    s.feed as feeds,
    (
      select feed_item->>'url'
      from jsonb_array_elements(s.feed) as feed_item
      where coalesce(feed_item->>'url', '') <> ''
        and coalesce((feed_item->>'isDefault')::boolean, false)
      limit 1
    ) as feed_url,
    s.sitemap as sitemap,
    s.link_page as link_page,
    s.recommend as featured,
    s.status as status,
    s.access_scope as access_scope,
    s.join_time as join_time,
    s.update_time as update_time,
    s.reason as reason,
    coalesce(article_stats.visible_articles, article_stats.total_articles, 0)::int as article_count,
    article_stats.latest_published_time as latest_published_time,
    coalesce(access_counters.total, 0)::int as visit_count,
    tag_summary.primary_tag as primary_tag,
    coalesce(tag_summary.sub_tags, '{}'::text[]) as sub_tags,
    coalesce(warning_summary.warning_tags, '[]'::jsonb) as warning_tags,
    coalesce(warning_summary.warning_names, '{}'::text[]) as warning_names,
    program_summary.program_id as program_id,
    program_summary.program_name as program_name,
    program_summary.program_is_open_source as program_is_open_source,
    program_summary.website_url as website_url,
    program_summary.repo_url as repo_url
  from sites s
  left join (
    select
      fa.site_id as site_id,
      count(*)::int as total_articles,
      count(*) filter (where fa.visibility = 'VISIBLE')::int as visible_articles,
      max(fa.published_time) as latest_published_time
    from feed_articles fa
    group by fa.site_id
  ) article_stats on article_stats.site_id = s.id
  left join (
    select
      sae.site_id as site_id,
      count(*)::int as total
    from site_access_events sae
    group by sae.site_id
  ) access_counters on access_counters.site_id = s.id
  left join (
    select
      st.site_id as site_id,
      min(td.name) filter (where td.tag_type = 'MAIN') as primary_tag,
      coalesce(
        array_agg(distinct td.name order by td.name) filter (where td.tag_type = 'SUB'),
        '{}'::text[]
      ) as sub_tags
    from site_tags st
    inner join tag_definitions td on td.id = st.tag_id
    where td.is_enabled = true
    group by st.site_id
  ) tag_summary on tag_summary.site_id = s.id
  left join (
    with warning_rows as (
      select distinct
        swt.site_id as site_id,
        td.machine_key as machine_key,
        td.name as name,
        td.description as description
      from site_warning_tags swt
      inner join tag_definitions td on td.id = swt.tag_id
      where td.tag_type = 'WARNING'
        and td.is_enabled = true
    )
    select
      warning_rows.site_id as site_id,
      coalesce(
        jsonb_agg(
          jsonb_build_object(
            'machineKey',
            warning_rows.machine_key,
            'name',
            warning_rows.name,
            'description',
            warning_rows.description
          )
          order by warning_rows.name
        ),
        '[]'::jsonb
      ) as warning_tags,
      coalesce(array_agg(warning_rows.name order by warning_rows.name), '{}'::text[]) as warning_names
    from warning_rows
    group by warning_rows.site_id
  ) warning_summary on warning_summary.site_id = s.id
  left join (
    select
      sa.site_id as site_id,
      p.id as program_id,
      p.name as program_name,
      p.is_open_source as program_is_open_source,
      p.website_url as website_url,
      p.repo_url as repo_url
    from site_architectures sa
    inner join programs p on p.id = sa.program_id
    where p.is_enabled = true
  ) program_summary on program_summary.site_id = s.id
  where s.is_show = true
);--> statement-breakpoint
CREATE VIEW "public"."site_access_counters" AS (
  select
    s.id as site_id,
    count(sae.id)::int as total,
    max(sae.occurred_time) as updated_time
  from sites s
  left join site_access_events sae on sae.site_id = s.id
  group by s.id
);--> statement-breakpoint
CREATE VIEW "public"."site_access_event_type_stats" AS (
  select
    sae.site_id as site_id,
    sae.event_type as event_type,
    count(*)::int as total,
    max(sae.occurred_time) as latest_access_time
  from site_access_events sae
  group by sae.site_id, sae.event_type
);--> statement-breakpoint
CREATE VIEW "public"."site_access_source_stats" AS (
  select
    sae.site_id as site_id,
    sae.event_type as event_type,
    sae.source as source,
    count(*)::int as total,
    max(sae.occurred_time) as latest_access_time
  from site_access_events sae
  group by sae.site_id, sae.event_type, sae.source
);--> statement-breakpoint
CREATE VIEW "public"."site_check_stats" AS (
  select
    scr.site_id as site_id,
    count(*)::int as total_checks,
    count(*) filter (where scr.derived_status = 'OK')::int as success_checks,
    count(*) filter (where scr.derived_status <> 'OK')::int as failed_checks,
    avg(scr.response_time_ms)::int as avg_response_time_ms
  from site_check_runs scr
  group by scr.site_id
);--> statement-breakpoint
CREATE VIEW "public"."site_feed_article_stats" AS (
  select
    fa.site_id as site_id,
    count(*)::int as total_articles,
    count(*) filter (where fa.visibility = 'VISIBLE')::int as visible_articles,
    max(fa.fetched_time) as latest_fetched_time,
    max(fa.published_time) as latest_published_time
  from feed_articles fa
  group by fa.site_id
);--> statement-breakpoint
CREATE VIEW "public"."site_program_summary_view" AS (
  select
    sa.site_id as site_id,
    p.id as program_id,
    p.name as program_name,
    p.is_open_source as program_is_open_source,
    p.website_url as website_url,
    p.repo_url as repo_url
  from site_architectures sa
  inner join programs p on p.id = sa.program_id
  where p.is_enabled = true
);--> statement-breakpoint
CREATE VIEW "public"."site_tag_summary_view" AS (
  select
    st.site_id as site_id,
    min(td.name) filter (where td.tag_type = 'MAIN') as primary_tag,
    coalesce(
      array_agg(distinct td.name order by td.name) filter (where td.tag_type = 'SUB'),
      '{}'::text[]
    ) as sub_tags
  from site_tags st
  inner join tag_definitions td on td.id = st.tag_id
  where td.is_enabled = true
  group by st.site_id
);--> statement-breakpoint
CREATE VIEW "public"."site_warning_tag_stats" AS (
  select
    td.id as tag_id,
    td.machine_key as machine_key,
    td.name as tag_name,
    count(distinct swt.site_id)::int as site_count
  from site_warning_tags swt
  inner join tag_definitions td on td.id = swt.tag_id
  group by td.id, td.machine_key, td.name
);--> statement-breakpoint
CREATE VIEW "public"."site_warning_tag_summary_view" AS (
  with warning_rows as (
    select distinct
      swt.site_id as site_id,
      td.machine_key as machine_key,
      td.name as name,
      td.description as description
    from site_warning_tags swt
    inner join tag_definitions td on td.id = swt.tag_id
    where td.tag_type = 'WARNING'
      and td.is_enabled = true
  )
  select
    warning_rows.site_id as site_id,
    coalesce(
      jsonb_agg(
        jsonb_build_object(
          'machineKey',
          warning_rows.machine_key,
          'name',
          warning_rows.name,
          'description',
          warning_rows.description
        )
        order by warning_rows.name
      ),
      '[]'::jsonb
    ) as warning_tags,
    coalesce(array_agg(warning_rows.name order by warning_rows.name), '{}'::text[]) as warning_names
  from warning_rows
  group by warning_rows.site_id
);--> statement-breakpoint
CREATE VIEW "public"."tag_stats" AS (
  select
    td.id as tag_id,
    td.name as tag_name,
    td.tag_type as tag_type,
    count(st.site_id)::int as site_count
  from tag_definitions td
  left join site_tags st on st.tag_id = td.id
  group by td.id, td.name, td.tag_type
);--> statement-breakpoint
CREATE VIEW "public"."technology_stats" AS (
  with technology_refs as (
    select distinct
      sa.site_id,
      pts.catalog_id as technology_id
    from site_architectures sa
    inner join program_technology_stacks pts on pts.program_id = sa.program_id
    where pts.catalog_id is not null
  )
  select
    tc.id as technology_id,
    tc.name as technology_name,
    tc.technology_type as technology_type,
    count(tr.site_id)::int as site_count
  from technology_catalogs tc
  left join technology_refs tr on tr.technology_id = tc.id
  group by tc.id, tc.name, tc.technology_type
);