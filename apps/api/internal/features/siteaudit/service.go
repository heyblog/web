package siteaudit

import (
	"log/slog"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/infrastructure/mail"
)

type Service struct {
	repository Store
	auth       *auth.Service
	newShortID func() (string, error)
	mailer     *mail.SubmissionMailer
	logger     *slog.Logger
}

type Dependencies struct {
	Repository Store
	Auth       *auth.Service
	NewShortID func() (string, error)
	Mailer     *mail.SubmissionMailer
	Logger     *slog.Logger
}

func NewService(dependencies Dependencies) *Service {
	return &Service{repository: dependencies.Repository, auth: dependencies.Auth, newShortID: dependencies.NewShortID, mailer: dependencies.Mailer, logger: dependencies.Logger}
}
