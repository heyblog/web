package mail

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

const utf8Charset = "UTF-8"

type sesClient interface {
	SendEmail(context.Context, *sesv2.SendEmailInput, ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

type sesSender struct {
	client sesClient
}

type sesSendError struct {
	cause error
}

func OpenSES(ctx context.Context, region string) (Sender, error) {
	configuration, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS configuration for SES: %w", err)
	}
	return newSESSender(sesv2.NewFromConfig(configuration)), nil
}

func newSESSender(client sesClient) Sender {
	return &sesSender{client: client}
}

func (sender *sesSender) Send(ctx context.Context, message Message) error {
	if err := message.validate(); err != nil {
		return fmt.Errorf("validate email message: %w", err)
	}

	body := &types.Body{}
	if strings.TrimSpace(message.Text) != "" {
		body.Text = content(message.Text)
	}
	if strings.TrimSpace(message.HTML) != "" {
		body.Html = content(message.HTML)
	}
	_, err := sender.client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(message.From),
		Destination: &types.Destination{
			ToAddresses: []string{message.To},
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: content(message.Subject),
				Body:    body,
			},
		},
	})
	if err != nil {
		return &sesSendError{cause: err}
	}
	return nil
}

func (err *sesSendError) Error() string {
	return "send email through AWS SES"
}

func (err *sesSendError) Unwrap() error {
	return err.cause
}

func content(value string) *types.Content {
	return &types.Content{
		Data:    aws.String(value),
		Charset: aws.String(utf8Charset),
	}
}
