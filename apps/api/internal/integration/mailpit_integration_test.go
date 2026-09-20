//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"heyblog-api/internal/mail"
)

const mailpitImage = "axllent/mailpit:v1.31.2@sha256:74d609a42ec279aa63c6b4622a6fa9b5408d1ad5b1d76a1c4be40a265ce0863d"

func TestMailpitSMTPTransport(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := testcontainers.Run(
		ctx,
		mailpitImage,
		testcontainers.WithExposedPorts("1025/tcp", "8025/tcp"),
		testcontainers.WithWaitStrategy(
			wait.ForHTTP("/readyz").WithPort("8025/tcp").WithStartupTimeout(time.Minute),
		),
	)
	if err != nil {
		t.Fatalf("start Mailpit container: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Errorf("terminate Mailpit container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("get Mailpit host: %v", err)
	}
	smtpPort, err := container.MappedPort(ctx, "1025/tcp")
	if err != nil {
		t.Fatalf("get Mailpit SMTP port: %v", err)
	}
	httpPort, err := container.MappedPort(ctx, "8025/tcp")
	if err != nil {
		t.Fatalf("get Mailpit HTTP port: %v", err)
	}

	sender, err := mail.OpenSMTP(ctx, net.JoinHostPort(host, smtpPort.Port()), 5*time.Second)
	if err != nil {
		t.Fatalf("open SMTP sender: %v", err)
	}
	want := mail.Message{
		From:    "sender@example.test",
		To:      "reader@example.test",
		Subject: "HeyBlog 集成测试",
		Text:    "纯文本邮件正文",
		HTML:    "<p>HTML 邮件正文</p>",
	}
	if err := sender.Send(ctx, want); err != nil {
		t.Fatalf("send Mailpit message: %v", err)
	}

	baseURL := "http://" + net.JoinHostPort(host, httpPort.Port())
	var listing struct {
		Messages []struct {
			ID      string `json:"ID"`
			Subject string `json:"Subject"`
			To      []struct {
				Address string `json:"Address"`
			} `json:"To"`
		} `json:"messages"`
	}
	listingPayload := getMailpitJSON(ctx, t, baseURL+"/api/v1/messages")
	if err := json.Unmarshal(listingPayload, &listing); err != nil {
		t.Fatalf("decode Mailpit message list: %v", err)
	}
	if len(listing.Messages) != 1 {
		t.Fatalf("Mailpit messages = %d, want 1", len(listing.Messages))
	}
	message := listing.Messages[0]
	if message.Subject != want.Subject || len(message.To) != 1 || message.To[0].Address != want.To {
		t.Fatalf("Mailpit summary = %#v, want expected subject and recipient", message)
	}

	var detail struct {
		Text string `json:"Text"`
		HTML string `json:"HTML"`
	}
	detailPayload := getMailpitJSON(ctx, t, baseURL+"/api/v1/message/"+message.ID)
	if err := json.Unmarshal(detailPayload, &detail); err != nil {
		t.Fatalf("decode Mailpit message detail: %v", err)
	}
	if detail.Text != want.Text || detail.HTML != want.HTML {
		t.Fatalf("Mailpit body = (%q, %q), want original alternatives", detail.Text, detail.HTML)
	}
}

func getMailpitJSON(ctx context.Context, t *testing.T, endpoint string) []byte {
	t.Helper()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatalf("create Mailpit request: %v", err)
	}
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatalf("request Mailpit API: %v", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("close Mailpit response: %v", err)
		}
	}()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("Mailpit API status = %d, want 200", response.StatusCode)
	}
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read Mailpit response from %q: %v", endpoint, err)
	}
	return payload
}
