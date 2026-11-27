package mail

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"
	"strings"
)

func renderSMTPMessage(message Message) ([]byte, error) {
	if err := message.validate(); err != nil {
		return nil, fmt.Errorf("validate email message: %w", err)
	}
	var buffer bytes.Buffer
	writeSMTPHeader(&buffer, "From", message.From)
	writeSMTPHeader(&buffer, "To", message.To)
	writeSMTPHeader(&buffer, "Subject", mime.QEncoding.Encode(utf8Charset, message.Subject))
	writeSMTPHeader(&buffer, "MIME-Version", "1.0")

	if strings.TrimSpace(message.Text) != "" && strings.TrimSpace(message.HTML) != "" {
		writer := multipart.NewWriter(&buffer)
		writeSMTPHeader(&buffer, "Content-Type", mime.FormatMediaType("multipart/alternative", map[string]string{"boundary": writer.Boundary()}))
		buffer.WriteString("\r\n")
		if err := writeSMTPPart(writer, "text/plain", message.Text); err != nil {
			return nil, err
		}
		if message.InlineImage == nil {
			if err := writeSMTPPart(writer, "text/html", message.HTML); err != nil {
				return nil, err
			}
		} else {
			var related bytes.Buffer
			contentType, err := writeSMTPRelated(&related, message)
			if err != nil {
				return nil, err
			}
			part, err := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {contentType}})
			if err != nil {
				return nil, fmt.Errorf("create SMTP related part: %w", err)
			}
			if _, err := part.Write(related.Bytes()); err != nil {
				return nil, fmt.Errorf("write SMTP related part: %w", err)
			}
		}
		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("close SMTP multipart body: %w", err)
		}
		return buffer.Bytes(), nil
	}
	if message.InlineImage != nil {
		var related bytes.Buffer
		contentType, err := writeSMTPRelated(&related, message)
		if err != nil {
			return nil, err
		}
		writeSMTPHeader(&buffer, "Content-Type", contentType)
		buffer.WriteString("\r\n")
		buffer.Write(related.Bytes())
		return buffer.Bytes(), nil
	}
	contentType, body := "text/plain", message.Text
	if strings.TrimSpace(message.HTML) != "" {
		contentType, body = "text/html", message.HTML
	}
	writeSMTPHeader(&buffer, "Content-Type", contentType+"; charset="+utf8Charset)
	writeSMTPHeader(&buffer, "Content-Transfer-Encoding", "quoted-printable")
	buffer.WriteString("\r\n")
	if err := writeSMTPText(&buffer, body); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeSMTPRelated(output io.Writer, message Message) (string, error) {
	writer := multipart.NewWriter(output)
	contentType := mime.FormatMediaType("multipart/related", map[string]string{"boundary": writer.Boundary(), "type": "text/html"})
	if err := writeSMTPPart(writer, "text/html", message.HTML); err != nil {
		return "", err
	}
	image := message.InlineImage
	header := textproto.MIMEHeader{
		"Content-Type":              {image.contentType},
		"Content-ID":                {"<" + image.contentID + ">"},
		"Content-Disposition":       {mime.FormatMediaType("inline", map[string]string{"filename": image.fileName})},
		"Content-Transfer-Encoding": {"base64"},
	}
	part, err := writer.CreatePart(header)
	if err != nil {
		return "", fmt.Errorf("create SMTP image part: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(image.data))
	for len(encoded) > 0 {
		end := min(76, len(encoded))
		if _, err := io.WriteString(part, encoded[:end]+"\r\n"); err != nil {
			return "", fmt.Errorf("write SMTP image: %w", err)
		}
		encoded = encoded[end:]
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close SMTP related body: %w", err)
	}
	return contentType, nil
}

func writeSMTPPart(writer *multipart.Writer, contentType, body string) error {
	header := textproto.MIMEHeader{
		"Content-Type":              {contentType + "; charset=" + utf8Charset},
		"Content-Transfer-Encoding": {"quoted-printable"},
	}
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("create SMTP MIME part: %w", err)
	}
	return writeSMTPText(part, body)
}

func writeSMTPText(output io.Writer, body string) error {
	writer := quotedprintable.NewWriter(output)
	if _, err := writer.Write([]byte(body)); err != nil {
		return fmt.Errorf("encode SMTP text: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close SMTP text encoder: %w", err)
	}
	return nil
}

func writeSMTPHeader(buffer *bytes.Buffer, name, value string) {
	buffer.WriteString(name)
	buffer.WriteString(": ")
	buffer.WriteString(value)
	buffer.WriteString("\r\n")
}
