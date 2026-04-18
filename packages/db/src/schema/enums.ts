import { pgEnum } from 'drizzle-orm/pg-core';

import { ANNOUNCEMENT_STATUS_KEYS } from '../constants/announcement';
import {
  ARTICLE_FEEDBACK_ACTION_KEYS,
  ARTICLE_FEEDBACK_REASON_KEYS,
  ARTICLE_VISIBILITY_KEYS,
  SITE_FEEDBACK_REASON_KEYS,
} from '../constants/article';
import { AUDIT_STATUS_KEYS, SITE_AUDIT_ACTION_KEYS } from '../constants/audit';
import { DEPLOYMENT_MODULE_KEYS, DEPLOYMENT_STATUS_KEYS } from '../constants/deployment';
import {
  SITE_CHECK_REGION_KEYS,
  SITE_CHECK_RESULT_KEYS,
  SITE_STATUS_TYPE_KEYS,
  SITE_WARNING_TAG_SOURCE_KEYS,
} from '../constants/monitoring';
import {
  FEED_TYPE_KEYS,
  FROM_SOURCE_KEYS,
  SITE_ACCESS_EVENT_TYPE_KEYS,
  SITE_ACCESS_SCOPE_KEYS,
  SITE_CLAIM_STATUS_KEYS,
  SITE_CLAIM_TYPE_KEYS,
  SITE_CLASSIFICATION_STATUS_KEYS,
} from '../constants/site';
import {
  CONTENT_VALIDATION_STATUS_KEYS,
  JOB_STATUS_KEYS,
  JOB_TRIGGER_SOURCE_KEYS,
  REQUEST_RETRY_STRATEGY_KEYS,
  REQUEST_TARGET_KIND_KEYS,
  RSS_FEED_FORMAT_KEYS,
  RSS_FEED_MODE_KEYS,
  RSS_FETCH_NETWORK_PATH_KEYS,
  RSS_FETCH_SOURCE_KIND_KEYS,
  RUN_RECORD_STATUS_KEYS,
  SCHEDULE_MODE_KEYS,
  SITE_CHECK_MODE_KEYS,
  SITE_VERIFY_RESULT_KEYS,
  TASK_TYPE_KEYS,
} from '../constants/task';
import {
  SITE_TECH_STACK_CATEGORY_KEYS,
  TAG_TYPE_KEYS,
  TECHNOLOGY_TYPE_KEYS,
} from '../constants/taxonomy';
import {
  MANAGEMENT_PERMISSION_KEYS,
  USER_OAUTH_PROVIDER_KEYS,
  USER_ROLE_KEYS,
} from '../constants/user';

export const fromSourceEnum = pgEnum('from_source_enum', FROM_SOURCE_KEYS as [string, ...string[]]);
export const fromSources = fromSourceEnum;
export const announcementStatusEnum = pgEnum(
  'announcement_status_enum',
  ANNOUNCEMENT_STATUS_KEYS as [string, ...string[]],
);
export const siteAccessScopeEnum = pgEnum(
  'site_access_scope_enum',
  SITE_ACCESS_SCOPE_KEYS as [string, ...string[]],
);
export const siteAccessEventTypeEnum = pgEnum(
  'site_access_event_type_enum',
  SITE_ACCESS_EVENT_TYPE_KEYS as [string, ...string[]],
);
export const siteClassificationStatusEnum = pgEnum(
  'site_classification_status_enum',
  SITE_CLASSIFICATION_STATUS_KEYS as [string, ...string[]],
);
export const siteClaimTypeEnum = pgEnum(
  'site_claim_type_enum',
  SITE_CLAIM_TYPE_KEYS as [string, ...string[]],
);
export const siteClaimStatusEnum = pgEnum(
  'site_claim_status_enum',
  SITE_CLAIM_STATUS_KEYS as [string, ...string[]],
);
export const articleVisibilityEnum = pgEnum(
  'article_visibility_enum',
  ARTICLE_VISIBILITY_KEYS as [string, ...string[]],
);
export const feedTypeEnum = pgEnum('feed_type_enum', FEED_TYPE_KEYS as [string, ...string[]]);
export const siteAuditActionEnum = pgEnum(
  'site_audit_action_enum',
  SITE_AUDIT_ACTION_KEYS as [string, ...string[]],
);
export const auditStatusEnum = pgEnum(
  'audit_status_enum',
  AUDIT_STATUS_KEYS as [string, ...string[]],
);
export const articleFeedbackActionEnum = pgEnum(
  'article_feedback_action_enum',
  ARTICLE_FEEDBACK_ACTION_KEYS as [string, ...string[]],
);
export const articleFeedbackReasonEnum = pgEnum(
  'article_feedback_reason_enum',
  ARTICLE_FEEDBACK_REASON_KEYS as [string, ...string[]],
);
export const siteFeedbackReasonEnum = pgEnum(
  'site_feedback_reason_enum',
  SITE_FEEDBACK_REASON_KEYS as [string, ...string[]],
);
export const taskTypeEnum = pgEnum('task_type_enum', TASK_TYPE_KEYS as [string, ...string[]]);
export const scheduleModeEnum = pgEnum(
  'schedule_mode_enum',
  SCHEDULE_MODE_KEYS as [string, ...string[]],
);
export const jobTriggerSourceEnum = pgEnum(
  'job_trigger_source_enum',
  JOB_TRIGGER_SOURCE_KEYS as [string, ...string[]],
);
export const jobStatusEnum = pgEnum('job_status_enum', JOB_STATUS_KEYS as [string, ...string[]]);
export const requestTargetKindEnum = pgEnum(
  'request_target_kind_enum',
  REQUEST_TARGET_KIND_KEYS as [string, ...string[]],
);
export const requestRetryStrategyEnum = pgEnum(
  'request_retry_strategy_enum',
  REQUEST_RETRY_STRATEGY_KEYS as [string, ...string[]],
);
export const runRecordStatusEnum = pgEnum(
  'run_record_status_enum',
  RUN_RECORD_STATUS_KEYS as [string, ...string[]],
);
export const siteVerifyResultEnum = pgEnum(
  'site_verify_result_enum',
  SITE_VERIFY_RESULT_KEYS as [string, ...string[]],
);
export const siteCheckModeEnum = pgEnum(
  'site_check_mode_enum',
  SITE_CHECK_MODE_KEYS as [string, ...string[]],
);
export const contentValidationStatusEnum = pgEnum(
  'content_validation_status_enum',
  CONTENT_VALIDATION_STATUS_KEYS as [string, ...string[]],
);
export const rssFeedModeEnum = pgEnum(
  'rss_feed_mode_enum',
  RSS_FEED_MODE_KEYS as [string, ...string[]],
);
export const rssFeedFormatEnum = pgEnum(
  'rss_feed_format_enum',
  RSS_FEED_FORMAT_KEYS as [string, ...string[]],
);
export const rssFetchSourceKindEnum = pgEnum(
  'rss_fetch_source_kind_enum',
  RSS_FETCH_SOURCE_KIND_KEYS as [string, ...string[]],
);
export const rssFetchNetworkPathEnum = pgEnum(
  'rss_fetch_network_path_enum',
  RSS_FETCH_NETWORK_PATH_KEYS as [string, ...string[]],
);
export const tagTypeEnum = pgEnum('tag_type_enum', TAG_TYPE_KEYS as [string, ...string[]]);
export const technologyTypeEnum = pgEnum(
  'technology_type_enum',
  TECHNOLOGY_TYPE_KEYS as [string, ...string[]],
);
export const siteTechStackCategoryEnum = pgEnum(
  'site_tech_stack_category_enum',
  SITE_TECH_STACK_CATEGORY_KEYS as [string, ...string[]],
);
export const userRoleEnum = pgEnum('user_role_enum', USER_ROLE_KEYS as [string, ...string[]]);
export const managementPermissionEnum = pgEnum(
  'management_permission_enum',
  MANAGEMENT_PERMISSION_KEYS,
);
export const userOauthProviderEnum = pgEnum(
  'user_oauth_provider_enum',
  USER_OAUTH_PROVIDER_KEYS as [string, ...string[]],
);
export const siteCheckRegionEnum = pgEnum(
  'site_check_region_enum',
  SITE_CHECK_REGION_KEYS as [string, ...string[]],
);
export const siteCheckResultEnum = pgEnum(
  'site_check_result_enum',
  SITE_CHECK_RESULT_KEYS as [string, ...string[]],
);
export const siteStatusTypeEnum = pgEnum(
  'site_status_type_enum',
  SITE_STATUS_TYPE_KEYS as [string, ...string[]],
);
export const siteWarningTagSourceEnum = pgEnum(
  'site_warning_tag_source_enum',
  SITE_WARNING_TAG_SOURCE_KEYS as [string, ...string[]],
);
export const deploymentStatusEnum = pgEnum(
  'deployment_status_enum',
  DEPLOYMENT_STATUS_KEYS as [string, ...string[]],
);
export const deploymentModuleEnum = pgEnum(
  'deployment_module_enum',
  DEPLOYMENT_MODULE_KEYS as [string, ...string[]],
);
