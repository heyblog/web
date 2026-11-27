package mail

import (
	"image/png"
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
	for _, want := range []string{"密码 &lt;重置&gt;", "立即重置", "token=abc&amp;next=1", "状态"} {
		if !strings.Contains(html.HTML, want) {
			t.Errorf("HTML does not contain %q", want)
		}
	}
}

func TestRenderDefaultEmailRejectsUnsafeActionURL(t *testing.T) {
	if _, err := RenderDefaultEmail(EmailTemplate{Title: "标题", Message: "正文", Action: &EmailAction{Label: "打开", URL: "javascript:alert(1)"}}); err == nil {
		t.Fatal("RenderDefaultEmail() error = nil, want unsafe URL error")
	}
}

func TestRenderDefaultEmailPreservesDetailLineBreaks(t *testing.T) {
	html, err := RenderDefaultEmail(EmailTemplate{Title: "审核结果", Message: "请检查以下意见。", Details: []EmailDetail{{Label: "审核意见", Value: "第一行\r\n第二行"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html.HTML, "第一行<br>第二行") {
		t.Fatal("review comment line breaks were lost")
	}
}

func TestRenderDefaultEmailIncludesDecodableInlineBrand(t *testing.T) {
	result, err := RenderDefaultEmail(EmailTemplate{Title: "验证码", Message: "请验证邮箱。"})
	if err != nil {
		t.Fatal(err)
	}
	if result.InlineImage == nil || !strings.Contains(result.HTML, `src="cid:`+result.InlineImage.contentID+`"`) {
		t.Fatal("brand image does not match the HTML CID")
	}
	config, err := png.DecodeConfig(strings.NewReader(result.InlineImage.data))
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != 96 || config.Height != 96 {
		t.Fatalf("brand dimensions = %dx%d", config.Width, config.Height)
	}
}

func TestRenderDefaultEmailEscapesDynamicFields(t *testing.T) {
	result, err := RenderDefaultEmail(EmailTemplate{Title: "<script>title</script>", Message: "<img src=x>\n&正文", Preheader: "<script>preview</script>", Details: []EmailDetail{{Label: "<b>label</b>", Value: "<b>first</b>\rsecond"}}, Action: &EmailAction{Label: "<b>action</b>", URL: "https://example.test/?a=1&b=2"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.HTML, "<script>") || strings.Contains(result.HTML, "<b>") || strings.Contains(result.HTML, "<img src=x>") {
		t.Fatal("dynamic HTML rendered without escaping")
	}
	if !strings.Contains(result.HTML, "&lt;b&gt;first&lt;/b&gt;<br>second") {
		t.Fatal("escaped multiline detail was lost")
	}
}

func TestRenderDefaultEmailRejectsEmptyActionLabel(t *testing.T) {
	if _, err := RenderDefaultEmail(EmailTemplate{Title: "标题", Message: "正文", Action: &EmailAction{Label: "  ", URL: "https://www.heyblog.net/"}}); err == nil {
		t.Fatal("empty action label accepted")
	}
}
