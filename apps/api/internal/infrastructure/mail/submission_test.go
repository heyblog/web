package mail

import (
	"context"
	"strings"
	"testing"
)

func TestSubmissionMailerKeepsCommentAndLookupGuidanceInBothAlternatives(t *testing.T) {
	for name, account := range map[string]bool{"account": true, "anonymous": false} {
		t.Run(name, func(t *testing.T) {
			sender := &recordingSender{}
			err := NewSubmissionMailer(sender, "sender@example.test").SendDecision(context.Background(), SubmissionDecision{Recipient: "reader@example.test", Action: "UPDATE", Status: "REJECTED", ReviewerComment: "第一条意见\n第二条意见", Account: account})
			if err != nil {
				t.Fatal(err)
			}
			message := sender.message
			if strings.Count(message.HTML, "第一条意见") != 1 || !strings.Contains(message.HTML, "第一条意见<br>第二条意见") {
				t.Fatal("HTML repeats the comment or loses its line breaks")
			}
			if !strings.Contains(message.Text, "第一条意见\n第二条意见") || !strings.Contains(message.Text, "修改申请未通过") {
				t.Fatal("plain text lost review information")
			}
			guidance := "查询凭证"
			if account {
				guidance = "我的提交"
			}
			if !strings.Contains(message.Text, guidance) || !strings.Contains(message.HTML, guidance) || message.InlineImage == nil {
				t.Fatal("lookup guidance or brand missing")
			}
		})
	}
}
