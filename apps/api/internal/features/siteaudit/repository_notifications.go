package siteaudit

import "context"

func (repository *Repository) DecisionRecipients(ctx context.Context, auditID string) ([]string, error) {
	id, err := parseUUID(auditID)
	if err != nil {
		return nil, err
	}
	return repository.queries.ListOwnerDecisionRecipients(ctx, id)
}
