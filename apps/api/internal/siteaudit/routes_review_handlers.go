package siteaudit

import (
	"context"

	"heyblog-api/internal/httpapi"
)

func managementReviewHandler(service *Service) func(context.Context, *auditIDBodyInput[ReviewInput]) (*bodyOutput[Audit], error) {
	return func(ctx context.Context, input *auditIDBodyInput[ReviewInput]) (*bodyOutput[Audit], error) {
		reviewer, err := service.CurrentReviewer(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, mapServiceError(err, "authorize site audit review")
		}
		input.Body.AuditID = input.AuditID
		audit, err := service.Review(ctx, reviewer, input.Body)
		if err != nil {
			return nil, mapServiceError(err, "review site audit")
		}
		return &bodyOutput[Audit]{Body: audit}, nil
	}
}

func managementSaveReviewDraftHandler(service *Service) func(context.Context, *auditIDBodyInput[ReviewDraftInput]) (*bodyOutput[Audit], error) {
	return func(ctx context.Context, input *auditIDBodyInput[ReviewDraftInput]) (*bodyOutput[Audit], error) {
		reviewer, err := service.CurrentReviewer(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, mapServiceError(err, "authorize site audit review draft")
		}
		input.Body.AuditID = input.AuditID
		audit, err := service.SaveReviewDraft(ctx, reviewer, input.Body)
		if err != nil {
			return nil, mapServiceError(err, "save site audit review draft")
		}
		return &bodyOutput[Audit]{Body: audit}, nil
	}
}

func managementDiscardReviewDraftHandler(service *Service) func(context.Context, *auditIDBodyInput[DiscardReviewDraftInput]) (*bodyOutput[Audit], error) {
	return func(ctx context.Context, input *auditIDBodyInput[DiscardReviewDraftInput]) (*bodyOutput[Audit], error) {
		reviewer, err := service.CurrentReviewer(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, mapServiceError(err, "authorize site audit review draft discard")
		}
		input.Body.AuditID = input.AuditID
		audit, err := service.DiscardReviewDraft(ctx, reviewer, input.Body)
		if err != nil {
			return nil, mapServiceError(err, "discard site audit review draft")
		}
		return &bodyOutput[Audit]{Body: audit}, nil
	}
}
