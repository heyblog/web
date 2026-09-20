package mail

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	netmail "net/mail"
	"strings"
	"testing"
	"time"
)

func TestRenderSMTPMessageBuildsUTF8MultipartAlternative(t *testing.T) {
	t.Parallel()

	message := Message{
		From:    "sender@example.test",
		To:      "reader@example.test",
		Subject: "HeyBlog 邮箱验证码",
		Text:    "纯文本验证码：123456",
		HTML:    "<p>HTML 验证码：<strong>123456</strong></p>",
	}
	payload, err := renderSMTPMessage(message)
	if err != nil {
		t.Fatalf("renderSMTPMessage() error = %v", err)
	}

	parsed, err := netmail.ReadMessage(strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(parsed.Header.Get("Subject"))
	if err != nil {
		t.Fatalf("DecodeHeader() error = %v", err)
	}
	if subject != message.Subject || parsed.Header.Get("From") != message.From || parsed.Header.Get("To") != message.To {
		t.Fatalf("headers = (%q, %q, %q), want original message headers", subject, parsed.Header.Get("From"), parsed.Header.Get("To"))
	}

	mediaType, parameters, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("ParseMediaType() error = %v", err)
	}
	if mediaType != "multipart/alternative" || parameters["boundary"] == "" {
		t.Fatalf("Content-Type = %q, want multipart/alternative with boundary", parsed.Header.Get("Content-Type"))
	}
	reader := multipart.NewReader(parsed.Body, parameters["boundary"])
	gotBodies := map[string]string{}
	for {
		part, partErr := reader.NextPart()
		if errors.Is(partErr, io.EOF) {
			break
		}
		if partErr != nil {
			t.Fatalf("NextPart() error = %v", partErr)
		}
		body, readErr := io.ReadAll(part)
		if readErr != nil {
			t.Fatalf("ReadAll() error = %v", readErr)
		}
		partType, _, parseErr := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if parseErr != nil {
			t.Fatalf("part Content-Type error = %v", parseErr)
		}
		gotBodies[partType] = string(body)
	}
	if gotBodies["text/plain"] != message.Text || gotBodies["text/html"] != message.HTML {
		t.Fatalf("message bodies = %#v, want text and HTML alternatives", gotBodies)
	}
}

func TestOpenSMTPProbesServerBeforeReturning(t *testing.T) {
	t.Parallel()

	address, completed := startSMTPProbeServer(t)
	sender, err := OpenSMTP(context.Background(), address, time.Second)
	if err != nil {
		t.Fatalf("OpenSMTP() error = %v", err)
	}
	if sender == nil {
		t.Fatal("OpenSMTP() sender = nil")
	}
	if err := <-completed; err != nil {
		t.Fatalf("SMTP probe server error = %v", err)
	}
}

func TestSMTPSenderWrapsDeliveryFailureWithoutMessageContents(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	sender := newSMTPSender(address, 100*time.Millisecond)
	err = sender.Send(context.Background(), Message{
		From:    "sender@example.test",
		To:      "private-recipient@example.test",
		Subject: "subject",
		Text:    "private verification code 123456",
	})
	if err == nil || !errors.Is(err, ErrDeliveryUnavailable) {
		t.Fatalf("Send() error = %v, want delivery-unavailable classification", err)
	}
	if strings.Contains(err.Error(), "private-recipient") || strings.Contains(err.Error(), "123456") {
		t.Fatalf("Send() error leaked message contents: %v", err)
	}
	component, ok := err.(interface{ Component() string })
	if !ok || component.Component() != "smtp" {
		t.Fatalf("Send() component = (%v, %t), want smtp", component, ok)
	}
}

func TestOpenSMTPHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := OpenSMTP(ctx, "127.0.0.1:1025", time.Second)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("OpenSMTP() error = %v, want context cancellation", err)
	}
}

func TestOpenSMTPClosesConnectionWhenQuitFails(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	connectionClosed := make(chan error, 1)
	go serveFailingSMTPQuit(listener, connectionClosed)

	if _, err := OpenSMTP(context.Background(), listener.Addr().String(), time.Second); err == nil {
		t.Fatal("OpenSMTP() error = nil, want SMTP QUIT error")
	}
	if err := <-connectionClosed; err != nil {
		t.Fatalf("SMTP connection was not closed after QUIT failure: %v", err)
	}
}

func startSMTPProbeServer(t *testing.T) (string, <-chan error) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	completed := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			completed <- acceptErr
			return
		}
		defer func() { _ = connection.Close() }()
		if _, writeErr := fmt.Fprint(connection, "220 mailpit.test ESMTP ready\r\n"); writeErr != nil {
			completed <- writeErr
			return
		}
		scanner := bufio.NewScanner(connection)
		for scanner.Scan() {
			command := strings.ToUpper(strings.Fields(scanner.Text())[0])
			switch command {
			case "EHLO", "HELO", "NOOP":
				_, err = fmt.Fprint(connection, "250 mailpit.test\r\n")
			case "QUIT":
				_, err = fmt.Fprint(connection, "221 bye\r\n")
				completed <- err
				return
			default:
				completed <- fmt.Errorf("unexpected SMTP command %q", command)
				return
			}
			if err != nil {
				completed <- err
				return
			}
		}
		completed <- scanner.Err()
	}()
	return listener.Addr().String(), completed
}

func serveFailingSMTPQuit(listener net.Listener, completed chan<- error) {
	connection, err := listener.Accept()
	if err != nil {
		completed <- err
		return
	}
	defer func() { _ = connection.Close() }()
	if _, err = fmt.Fprint(connection, "220 mailpit.test ESMTP ready\r\n"); err != nil {
		completed <- err
		return
	}
	scanner := bufio.NewScanner(connection)
	for scanner.Scan() {
		command := strings.ToUpper(strings.Fields(scanner.Text())[0])
		switch command {
		case "EHLO", "HELO", "NOOP":
			_, err = fmt.Fprint(connection, "250 mailpit.test\r\n")
		case "QUIT":
			if _, err = fmt.Fprint(connection, "500 quit failed\r\n"); err == nil {
				if scanner.Scan() {
					err = fmt.Errorf("unexpected SMTP command after QUIT: %q", scanner.Text())
				} else {
					err = scanner.Err()
				}
			}
			completed <- err
			return
		default:
			completed <- fmt.Errorf("unexpected SMTP command %q", command)
			return
		}
		if err != nil {
			completed <- err
			return
		}
	}
	completed <- scanner.Err()
}
