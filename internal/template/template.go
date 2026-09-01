package template

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
)

const (
	CerberusInvitation     = "cerberus.organization-invitation"
	CerberusBetaInvitation = "cerberus.beta-invitation"
	CerberusOpsInvitation  = "cerberus.ops-invitation"
	ObsDogInvitation       = "obsdog.organization-invitation"
)

type Request struct {
	Template  string            `json:"template"`
	Recipient string            `json:"recipient"`
	Locale    string            `json:"locale"`
	Variables map[string]string `json:"variables"`
}

type Message struct{ Recipient, FromAddress, FromName, Subject, Text, SESConfigurationSet string }

func Render(request Request) (Message, error) {
	address, err := mail.ParseAddress(strings.TrimSpace(request.Recipient))
	if err != nil || address.Address != strings.TrimSpace(request.Recipient) || strings.ContainsAny(address.Address, "\r\n") {
		return Message{}, errors.New("invalid recipient")
	}
	if (request.Locale != "en" && request.Locale != "ko") || len(request.Variables) != 1 {
		return Message{}, errors.New("invalid template request")
	}
	link := request.Variables["invitationLink"]
	parsed, err := url.Parse(link)
	if err != nil || parsed.Scheme != "https" || parsed.RawQuery != "" {
		return Message{}, errors.New("invalid invitation link")
	}
	switch request.Template {
	case CerberusInvitation:
		if parsed.Host != "console.c6s.whitekiwi.link" || parsed.Path != "/invitations/accept/" || !validTokenFragment(parsed.Fragment) {
			return Message{}, errors.New("invalid invitation link")
		}
		if request.Locale == "ko" {
			return Message{Recipient: address.Address, FromName: "Cerberus", Subject: "Cerberus 조직에 초대되었습니다", Text: fmt.Sprintf("Cerberus 조직 초대를 확인하고 수락하려면 아래 링크를 여세요.\r\n\r\n%s\r\n\r\n이 초대는 7일 후 만료됩니다. 요청한 적이 없다면 이 메일을 무시하세요.\r\n", link)}, nil
		}
		return Message{Recipient: address.Address, FromName: "Cerberus", Subject: "You were invited to a Cerberus organization", Text: fmt.Sprintf("Open the link below to review and accept your Cerberus organization invitation.\r\n\r\n%s\r\n\r\nThis invitation expires in 7 days. Ignore this email if you did not expect it.\r\n", link)}, nil
	case CerberusBetaInvitation:
		if parsed.Host != "console.c6s.whitekiwi.link" || parsed.Path != "/signup/" || parsed.Fragment != "" {
			return Message{}, errors.New("invalid invitation link")
		}
		if request.Locale == "ko" {
			return Message{Recipient: address.Address, FromName: "Cerberus", Subject: "Cerberus 비공개 베타에 초대되었습니다", Text: fmt.Sprintf("신청이 승인되었습니다. 초대받은 이 Google 또는 Apple 이메일로 가입을 완료하세요.\r\n\r\n%s\r\n\r\n초대는 30일 후 만료됩니다. 요청한 적이 없다면 이 메일을 무시하세요.\r\n", link)}, nil
		}
		return Message{Recipient: address.Address, FromName: "Cerberus", Subject: "Your Cerberus private beta invitation", Text: fmt.Sprintf("Your application was approved. Complete signup with this invited Google or Apple email.\r\n\r\n%s\r\n\r\nThe invitation expires in 30 days. Ignore this email if you did not request access.\r\n", link)}, nil
	case CerberusOpsInvitation:
		if parsed.Host != "ops.c6s.whitekiwi.link" || parsed.Path != "/ops/" || parsed.Fragment != "" {
			return Message{}, errors.New("invalid invitation link")
		}
		if request.Locale == "ko" {
			return Message{Recipient: address.Address, FromName: "Cerberus", Subject: "Cerberus 운영자로 초대되었습니다", Text: fmt.Sprintf("초대받은 Google 이메일로 Ops에 로그인하세요.\r\n\r\n%s\r\n\r\n초대는 30일 후 만료됩니다. 요청한 적이 없다면 이 메일을 무시하세요.\r\n", link)}, nil
		}
		return Message{Recipient: address.Address, FromName: "Cerberus", Subject: "You were invited to Cerberus Ops", Text: fmt.Sprintf("Sign in to Ops with this invited Google email.\r\n\r\n%s\r\n\r\nThe invitation expires in 30 days. Ignore this email if you did not expect it.\r\n", link)}, nil
	case ObsDogInvitation:
		if parsed.Host != "app.obsdog.ai" || parsed.Path != "/invitations/accept/" || !validTokenFragment(parsed.Fragment) {
			return Message{}, errors.New("invalid invitation link")
		}
		if request.Locale == "ko" {
			return Message{Recipient: address.Address, FromName: "ObsDog", Subject: "ObsDog Organization에 초대되었습니다", Text: fmt.Sprintf("초대된 Organization과 Space, 권한을 확인하고 수락하려면 아래 링크를 여세요.\r\n\r\n%s\r\n\r\n이 초대는 7일 후 만료됩니다. 요청한 적이 없다면 이 메일을 무시하세요.\r\n", link)}, nil
		}
		return Message{Recipient: address.Address, FromName: "ObsDog", Subject: "You were invited to an ObsDog organization", Text: fmt.Sprintf("Open the link below to review the organization, Space, and role before accepting.\r\n\r\n%s\r\n\r\nThis invitation expires in 7 days. Ignore this email if you did not expect it.\r\n", link)}, nil
	default:
		return Message{}, errors.New("invalid template request")
	}
}

func validTokenFragment(fragment string) bool {
	if !strings.HasPrefix(fragment, "token=") || len(fragment) < len("token=")+32 || len(fragment) > len("token=")+256 {
		return false
	}
	return !strings.ContainsAny(strings.TrimPrefix(fragment, "token="), "&?#\r\n")
}
