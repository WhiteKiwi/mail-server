package template

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
)

const CerberusInvitation = "cerberus.organization-invitation"

type Request struct {
	Template  string            `json:"template"`
	Recipient string            `json:"recipient"`
	Locale    string            `json:"locale"`
	Variables map[string]string `json:"variables"`
}

type Message struct{ Recipient, FromName, Subject, Text string }

func Render(request Request) (Message, error) {
	address, err := mail.ParseAddress(strings.TrimSpace(request.Recipient))
	if err != nil || address.Address != strings.TrimSpace(request.Recipient) || strings.ContainsAny(address.Address, "\r\n") {
		return Message{}, errors.New("invalid recipient")
	}
	if request.Template != CerberusInvitation || (request.Locale != "en" && request.Locale != "ko") || len(request.Variables) != 1 {
		return Message{}, errors.New("invalid template request")
	}
	link := request.Variables["invitationLink"]
	parsed, err := url.Parse(link)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "console.c6s.whitekiwi.link" || parsed.Path != "/invitations/accept/" || parsed.RawQuery != "" || !strings.HasPrefix(parsed.Fragment, "token=") {
		return Message{}, errors.New("invalid invitation link")
	}
	if request.Locale == "ko" {
		return Message{address.Address, "Cerberus", "Cerberus 조직에 초대되었습니다", fmt.Sprintf("Cerberus 조직 초대를 확인하고 수락하려면 아래 링크를 여세요.\r\n\r\n%s\r\n\r\n이 초대는 7일 후 만료됩니다. 요청한 적이 없다면 이 메일을 무시하세요.\r\n", link)}, nil
	}
	return Message{address.Address, "Cerberus", "You were invited to a Cerberus organization", fmt.Sprintf("Open the link below to review and accept your Cerberus organization invitation.\r\n\r\n%s\r\n\r\nThis invitation expires in 7 days. Ignore this email if you did not expect it.\r\n", link)}, nil
}
