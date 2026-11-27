package mail

import (
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	netmail "net/mail"
	"strings"
	"testing"
)

type decodedMIMEPart struct {
	mediaType   string
	contentID   string
	disposition string
	body        string
	children    []decodedMIMEPart
}

func decodeMIMEPart(t *testing.T, header netmail.Header, body io.Reader) decodedMIMEPart {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	result := decodedMIMEPart{mediaType: mediaType, contentID: header.Get("Content-ID"), disposition: header.Get("Content-Disposition")}
	if strings.HasPrefix(mediaType, "multipart/") {
		reader := multipart.NewReader(body, params["boundary"])
		for {
			part, err := reader.NextRawPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			result.children = append(result.children, decodeMIMEPart(t, netmail.Header(part.Header), part))
		}
		return result
	}
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	body = strings.NewReader(string(raw))
	switch header.Get("Content-Transfer-Encoding") {
	case "quoted-printable":
		body = quotedprintable.NewReader(body)
	case "base64":
		for _, line := range strings.Split(string(raw), "\r\n") {
			if len(line) > 76 {
				t.Fatalf("base64 line exceeds 76 bytes: %d", len(line))
			}
		}
		body = base64.NewDecoder(base64.StdEncoding, body)
	default:
		t.Fatalf("unencoded MIME body: %q", header.Get("Content-Transfer-Encoding"))
	}
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	result.body = string(data)
	return result
}

func TestRenderSMTPMessageEmbedsBrandInHTMLAlternative(t *testing.T) {
	rendered, err := RenderDefaultEmail(EmailTemplate{Title: "邮件", Message: strings.Repeat("很长的中文正文", 200)})
	if err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"both": "纯文本内容", "HTML only": ""} {
		t.Run(name, func(t *testing.T) {
			payload, err := renderSMTPMessage(Message{From: "sender@example.test", To: "reader@example.test", Subject: "邮件", Text: text, HTML: rendered.HTML, InlineImage: rendered.InlineImage})
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := netmail.ReadMessage(strings.NewReader(string(payload)))
			if err != nil {
				t.Fatal(err)
			}
			root := decodeMIMEPart(t, parsed.Header, parsed.Body)
			if text != "" {
				if root.mediaType != "multipart/alternative" || len(root.children) != 2 || root.children[0].mediaType != "text/plain" || root.children[0].body != text {
					t.Fatal("plain text must precede the HTML alternative")
				}
				root = root.children[1]
			}
			if root.mediaType != "multipart/related" || len(root.children) != 2 {
				t.Fatal("HTML and image must share a related part")
			}
			html, image := root.children[0], root.children[1]
			if html.mediaType != "text/html" || normalizeEmailText(html.body) != normalizeEmailText(rendered.HTML) {
				t.Fatal("HTML body did not survive MIME encoding")
			}
			if image.mediaType != "image/png" || image.contentID != "<"+rendered.InlineImage.contentID+">" || image.body != rendered.InlineImage.data || !strings.HasPrefix(image.disposition, "inline;") {
				t.Fatal("inline image bytes or headers did not survive MIME encoding")
			}
			_, rawBody, ok := strings.Cut(string(payload), "\r\n\r\n")
			if !ok {
				t.Fatal("missing MIME header separator")
			}
			for _, line := range strings.Split(rawBody, "\r\n") {
				if len(line) > 998 {
					t.Fatalf("SMTP body line exceeds the 998-byte limit: %d", len(line))
				}
			}
		})
	}
}

func TestRenderSMTPMessageEncodesLongStandaloneBodies(t *testing.T) {
	longBody := strings.Repeat("中文 & <text>", 200)
	for name, message := range map[string]Message{"text": {Text: longBody}, "HTML": {HTML: longBody}} {
		t.Run(name, func(t *testing.T) {
			message.From, message.To, message.Subject = "sender@example.test", "reader@example.test", "邮件"
			payload, err := renderSMTPMessage(message)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := netmail.ReadMessage(strings.NewReader(string(payload)))
			if err != nil {
				t.Fatal(err)
			}
			decoded := decodeMIMEPart(t, parsed.Header, parsed.Body)
			if decoded.body != longBody {
				t.Fatal("standalone body changed during encoding")
			}
			_, rawBody, _ := strings.Cut(string(payload), "\r\n\r\n")
			for _, line := range strings.Split(rawBody, "\r\n") {
				if len(line) > 76 {
					t.Fatalf("encoded body line is %d bytes", len(line))
				}
			}
		})
	}
}
