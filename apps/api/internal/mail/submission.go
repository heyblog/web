package mail

import (
	"context"
	"fmt"
	"strings"
)

type SubmissionDecision struct {
	Recipient       string
	Action          string
	Status          string
	ReviewerComment string
}

type SubmissionMailer struct {
	sender Sender
	from   string
}

func NewSubmissionMailer(sender Sender, from string) *SubmissionMailer {
	return &SubmissionMailer{sender: sender, from: from}
}

func (mailer *SubmissionMailer) SendDecision(ctx context.Context, decision SubmissionDecision) error {
	if strings.TrimSpace(decision.Recipient) == "" {
		return fmt.Errorf("submission decision recipient is required")
	}
	statusLabel := "已通过"
	if decision.Status == "REJECTED" {
		statusLabel = "未通过"
	}
	text := fmt.Sprintf("您的站点%s申请%s。", actionLabel(decision.Action), statusLabel)
	if comment := strings.TrimSpace(decision.ReviewerComment); comment != "" {
		text += "\n\n审核意见：\n" + comment
	}
	text += "\n\n请使用提交时保存的查询凭证查看完整状态。"
	if err := mailer.sender.Send(ctx, Message{From: mailer.from, To: decision.Recipient, Subject: "HeyBlog 站点申请审核结果", Text: text}); err != nil {
		return fmt.Errorf("send submission decision email: %w", err)
	}
	return nil
}

func actionLabel(action string) string {
	switch action {
	case "CREATE":
		return "新增"
	case "UPDATE":
		return "修改"
	case "DELETE":
		return "删除"
	case "RESTORE":
		return "恢复"
	default:
		return ""
	}
}
