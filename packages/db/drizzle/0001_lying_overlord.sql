ALTER TYPE "public"."deployment_status_enum" ADD VALUE 'SELF_UPDATING' BEFORE 'SUCCESS';--> statement-breakpoint
ALTER TYPE "public"."deployment_status_enum" ADD VALUE 'WAITING_RESUME' BEFORE 'SUCCESS';--> statement-breakpoint
ALTER TYPE "public"."deployment_status_enum" ADD VALUE 'RUNNING_INFRA_SYNC' BEFORE 'SUCCESS';--> statement-breakpoint
ALTER TYPE "public"."deployment_status_enum" ADD VALUE 'RUNNING_DB_MIGRATE' BEFORE 'SUCCESS';--> statement-breakpoint
ALTER TYPE "public"."deployment_status_enum" ADD VALUE 'RUNNING_SERVICES' BEFORE 'SUCCESS';