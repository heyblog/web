package mail

import (
	"strings"
	"testing"
)

func TestRenderDefaultEmailEscapesContentAndRendersAction(t *testing.T) {
	html, err := RenderDefaultEmail(EmailTemplate{
		Title:   "密码 <重置>",
		Message: "请继续。\n\n这是一段说明。",
		Details: []EmailDetail{{Label: "状态", Value: "待处理"}},
		Action:  &EmailAction{Label: "立即重置", URL: "https://www.heyblog.net/reset-password?token=abc&next=1"},
	})
	if err != nil {
		t.Fatalf("RenderDefaultEmail() error = %v", err)
	}
	for _, want := range []string{"background:#09090b", "#158187", "密码 &lt;重置&gt;", "立即重置", "token=abc&amp;next=1", "状态"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML does not contain %q", want)
		}
	}
}

func TestRenderDefaultEmailRejectsUnsafeActionURL(t *testing.T) {
	if _, err := RenderDefaultEmail(EmailTemplate{Title: "标题", Message: "正文", Action: &EmailAction{Label: "打开", URL: "javascript:alert(1)"}}); err == nil {
		t.Fatal("RenderDefaultEmail() error = nil, want unsafe URL error")
	}
}
