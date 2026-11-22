package mail

import (
	"fmt"
	"html"
	"net/url"
	"strings"
)

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

func RenderDefaultEmail(input EmailTemplate) (string, error) {
	title := strings.TrimSpace(input.Title)
	message := normalizeEmailText(input.Message)
	if title == "" || message == "" {
		return "", fmt.Errorf("email title and message are required")
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
		body.WriteString(`<p style="margin:0 0 16px;color:#3f3f46;font-size:16px;line-height:1.75;">`)
		body.WriteString(strings.ReplaceAll(html.EscapeString(paragraph), "\n", "<br>"))
		body.WriteString(`</p>`)
	}

	var details strings.Builder
	for _, detail := range input.Details {
		label, value := strings.TrimSpace(detail.Label), strings.TrimSpace(detail.Value)
		if label == "" || value == "" {
			continue
		}
		details.WriteString(`<tr><td style="padding:10px 12px;color:#71717a;font-size:14px;border-bottom:1px solid #e4e4e7;vertical-align:top;">`)
		details.WriteString(html.EscapeString(label))
		details.WriteString(`</td><td style="padding:10px 12px;color:#27272a;font-size:14px;border-bottom:1px solid #e4e4e7;vertical-align:top;">`)
		details.WriteString(html.EscapeString(value))
		details.WriteString(`</td></tr>`)
	}

	action := ""
	if input.Action != nil {
		actionURL, err := emailActionURL(input.Action.URL)
		if err != nil {
			return "", err
		}
		action = `<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin:24px 0 8px;"><tr><td bgcolor="#158187" style="border-radius:6px;"><a href="` + html.EscapeString(actionURL) + `" style="display:inline-block;padding:12px 20px;color:#ffffff;font-size:15px;font-weight:700;line-height:20px;text-decoration:none;">` + html.EscapeString(strings.TrimSpace(input.Action.Label)) + `</a></td></tr></table>`
	}

	return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light"><title>` + html.EscapeString(title) + `</title></head><body style="margin:0;padding:0;background:#f4f4f5;font-family:Inter,'Noto Sans SC','Segoe UI',Arial,sans-serif;"><div style="display:none;max-height:0;overflow:hidden;opacity:0;color:transparent;">` + html.EscapeString(preheader) + `</div><table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" style="width:100%;background:#f4f4f5;"><tr><td align="center" style="padding:32px 16px;"><table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" style="width:100%;max-width:600px;background:#ffffff;border:1px solid #e4e4e7;border-radius:8px;overflow:hidden;"><tr><td style="padding:24px 32px;background:#09090b;border-bottom:4px solid #158187;"><a href="https://www.heyblog.net/" style="color:#f4f4f5;font-size:24px;font-weight:800;line-height:32px;text-decoration:none;">HeyBlog</a><div style="margin-top:4px;color:#a1a1aa;font-size:13px;line-height:20px;">独立博客网络</div></td></tr><tr><td style="padding:32px;"><h1 style="margin:0 0 20px;color:#18181b;font-size:24px;line-height:1.4;font-weight:750;">` + html.EscapeString(title) + `</h1>` + body.String() + func() string {
		if details.Len() == 0 {
			return ""
		}
		return `<table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" style="margin:20px 0;border:1px solid #e4e4e7;border-radius:6px;border-collapse:separate;border-spacing:0;">` + details.String() + `</table>`
	}() + action + `</td></tr><tr><td style="padding:20px 32px;background:#fafafa;border-top:1px solid #e4e4e7;color:#71717a;font-size:12px;line-height:20px;">这封邮件由 <a href="https://www.heyblog.net/" style="color:#0f7b80;text-decoration:none;">HeyBlog</a> 发出。请勿向任何人提供密码或验证码。</td></tr></table></td></tr></table></body></html>`, nil
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
