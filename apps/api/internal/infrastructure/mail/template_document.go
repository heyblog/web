package mail

// Email clients receive resolved Aria light tokens as inline CSS: slate canvas,
// surface, text and dividers, teal primary, 24px card padding and 6px radius.
// Keep layout tables presentational; the details table retains row semantics.
const emailDocument = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light"><title>%s</title></head>
<body style="margin:0;padding:0;background:#f8fafc;color:#0f172a;font-family:Inter,'Noto Sans SC','Segoe UI',Arial,sans-serif;">
<div aria-hidden="true" style="display:none;max-height:0;overflow:hidden;opacity:0;color:transparent;mso-hide:all;">%s</div>
<table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="width:100%%;background:#f8fafc;"><tr><td align="center" style="padding:24px 16px;">
<!--[if mso]><table role="presentation" width="600" cellspacing="0" cellpadding="0" border="0"><tr><td><![endif]-->
<table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="width:100%%;max-width:600px;table-layout:fixed;background:#ffffff;border:1px solid #e2e8f0;border-radius:6px;">
<tr><td style="padding:24px;border-bottom:1px solid #e2e8f0;">
<a href="https://www.heyblog.net/" style="display:inline-block;color:#0f172a;font-size:20px;font-weight:600;line-height:44px;text-decoration:none;"><img src="cid:%s" alt="" width="40" height="40" style="display:inline-block;width:40px;height:40px;margin-right:12px;border:0;vertical-align:middle;">HeyBlog</a>
</td></tr>
<tr><td style="padding:24px;"><h1 style="margin:0 0 24px;color:#0f172a;font-size:24px;line-height:32px;font-weight:600;overflow-wrap:anywhere;word-break:break-word;">%s</h1>%s%s%s</td></tr>
<tr><td style="padding:24px;border-top:1px solid #e2e8f0;color:#64748b;font-size:12px;line-height:18px;">这封邮件由 <a href="https://www.heyblog.net/" style="color:#0c666b;text-decoration:underline;">HeyBlog</a> 发出。请勿向任何人提供密码或验证码。</td></tr>
</table>
<!--[if mso]></td></tr></table><![endif]-->
</td></tr></table></body></html>`
