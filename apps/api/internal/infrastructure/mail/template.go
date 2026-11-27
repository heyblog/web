package mail

import (
	_ "embed"
	"fmt"
	"html"
	"net/url"
	"strings"
)

const brandContentID = "heyblog-brand"

// brand.png is a 96px raster of apps/web/public/favicon.svg for email clients.
//
//go:embed brand.png
var brandImage string

type RenderedEmail struct {
	HTML        string
	InlineImage *InlineImage
}

// EmailTemplate contains the content rendered by the shared HeyBlog email shell.
type EmailTemplate struct {
	Title     string
	Message   string
	Preheader string
	Details   []EmailDetail
	Action    *EmailAction
}

type EmailDetail struct {
	Label string
	Value string
}

type EmailAction struct {
	Label string
	URL   string
}

func RenderDefaultEmail(input EmailTemplate) (RenderedEmail, error) {
	title := strings.TrimSpace(input.Title)
	message := normalizeEmailText(input.Message)
	if title == "" || message == "" {
		return RenderedEmail{}, fmt.Errorf("email title and message are required")
	}
	preheader := normalizeEmailText(input.Preheader)
	if preheader == "" {
		preheader = message
	}

	paragraphs := strings.Split(message, "\n\n")
	var body strings.Builder
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		body.WriteString(`<p style="margin:0 0 16px;color:#0f172a;font-size:16px;line-height:24px;overflow-wrap:anywhere;word-break:break-word;">`)
		body.WriteString(strings.ReplaceAll(html.EscapeString(paragraph), "\n", "<br>"))
		body.WriteString(`</p>`)
	}

	var details strings.Builder
	for _, detail := range input.Details {
		label, value := strings.TrimSpace(detail.Label), normalizeEmailText(detail.Value)
		if label == "" || value == "" {
			continue
		}
		details.WriteString(`<tr><th scope="row" width="88" style="width:88px;padding:12px 12px 12px 0;color:#64748b;font-size:14px;font-weight:500;line-height:21px;text-align:left;border-bottom:1px solid #e2e8f0;vertical-align:top;">`)
		details.WriteString(html.EscapeString(label))
		details.WriteString(`</th><td style="padding:12px 0;color:#0f172a;font-size:14px;line-height:21px;border-bottom:1px solid #e2e8f0;vertical-align:top;overflow-wrap:anywhere;word-break:break-word;">`)
		details.WriteString(strings.ReplaceAll(html.EscapeString(value), "\n", "<br>"))
		details.WriteString(`</td></tr>`)
	}

	action := ""
	if input.Action != nil {
		label := strings.TrimSpace(input.Action.Label)
		if label == "" {
			return RenderedEmail{}, fmt.Errorf("email action label is required")
		}
		actionURL, err := emailActionURL(input.Action.URL)
		if err != nil {
			return RenderedEmail{}, err
		}
		action = `<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin:24px 0 16px;"><tr><td bgcolor="#0f7b80" style="border-radius:6px;text-align:center;"><a href="` + html.EscapeString(actionURL) + `" style="display:inline-block;padding:12px 24px;border:1px solid #0f7b80;border-radius:6px;color:#ffffff;font-size:16px;font-weight:500;line-height:24px;text-decoration:none;">` + html.EscapeString(label) + `</a></td></tr></table><p style="margin:0;font-size:12px;line-height:18px;color:#64748b;overflow-wrap:anywhere;word-break:break-word;">也可复制此链接到浏览器：<br><a href="` + html.EscapeString(actionURL) + `" style="color:#0c666b;text-decoration:underline;">` + html.EscapeString(actionURL) + `</a></p>`
	}

	detailTable := ""
	if details.Len() > 0 {
		detailTable = `<table width="100%" cellspacing="0" cellpadding="0" border="0" style="width:100%;table-layout:fixed;margin:8px 0 24px;border-collapse:collapse;">` + details.String() + `</table>`
	}
	return RenderedEmail{
		HTML:        fmt.Sprintf(emailDocument, html.EscapeString(title), html.EscapeString(preheader), brandContentID, html.EscapeString(title), body.String(), detailTable, action),
		InlineImage: &InlineImage{contentID: brandContentID, fileName: "heyblog.png", contentType: "image/png", data: brandImage},
	}, nil
}

func normalizeEmailText(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n"))
}

func emailActionURL(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("email action URL must use HTTP or HTTPS")
	}
	return parsed.String(), nil
}
