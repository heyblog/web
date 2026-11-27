package sitemanagement

import (
	"context"
	"net/http"

	"heyblog-api/internal/platform/httpapi"
)

func startCheckHandler(s *Service) func(context.Context, *claimRequest) (*output[Claim], error) {
	return func(ctx context.Context, in *claimRequest) (*output[Claim], error) {
		user, err := s.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, routeError(err)
		}
		claim, err := s.StartCheck(ctx, user, in.ID)
		if err != nil {
			return nil, routeError(err)
		}
		result := response(claim)
		if claim.Task != nil {
			result.Status = http.StatusAccepted
		}
		return result, nil
	}
}

func claimTaskHandler(s *Service) func(context.Context, *claimRequest) (*output[*VerificationTask], error) {
	return func(ctx context.Context, in *claimRequest) (*output[*VerificationTask], error) {
		user, err := s.Current(ctx, httpapi.Request(ctx))
		if err != nil {
			return nil, routeError(err)
		}
		if _, err := s.applicant(ctx, user, in.ID); err != nil {
			return nil, routeError(err)
		}
		if s.queue == nil {
			return response[*VerificationTask](nil), nil
		}
		task, err := s.queue.ClaimTask(ctx, in.ID, user.ID)
		if err != nil {
			return nil, routeError(err)
		}
		return response(task), nil
	}
}
