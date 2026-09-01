package template

import "testing"

func TestCerberusInvitationRequiresFixedConsoleFragmentLink(t *testing.T) {
	valid := Request{Template: CerberusInvitation, Recipient: "person@example.com", Locale: "en", Variables: map[string]string{"invitationLink": "https://console.c6s.whitekiwi.link/invitations/accept/#token=abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"}}
	message, err := Render(valid)
	if err != nil {
		t.Fatal(err)
	}
	if message.Recipient != "person@example.com" || message.Subject == "" || message.Text == "" {
		t.Fatalf("message = %#v", message)
	}
	for _, link := range []string{"https://evil.example/#token=x", "https://console.c6s.whitekiwi.link/invitations/accept/?token=x", "https://console.c6s.whitekiwi.link/invitations/accept/#other=x"} {
		copy := valid
		copy.Variables = map[string]string{"invitationLink": link}
		if _, err := Render(copy); err == nil {
			t.Fatalf("accepted %q", link)
		}
	}
}

func TestTemplateRejectsHeaderInjectionAndUnknownVariables(t *testing.T) {
	request := Request{Template: CerberusInvitation, Recipient: "person@example.com\r\nBcc: attacker@example.com", Locale: "en", Variables: map[string]string{"invitationLink": "https://console.c6s.whitekiwi.link/invitations/accept/#token=abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"}}
	if _, err := Render(request); err == nil {
		t.Fatal("accepted recipient injection")
	}
	request.Recipient = "person@example.com"
	request.Variables["unexpected"] = "value"
	if _, err := Render(request); err == nil {
		t.Fatal("accepted unknown template variable")
	}
}

func TestCerberusBetaAndOpsInvitationsUseFixedOrigins(t *testing.T) {
	for _, test := range []struct{ template, link string }{
		{CerberusBetaInvitation, "https://console.c6s.whitekiwi.link/signup/"},
		{CerberusOpsInvitation, "https://ops.c6s.whitekiwi.link/ops/"},
	} {
		request := Request{Template: test.template, Recipient: "person@example.com", Locale: "ko", Variables: map[string]string{"invitationLink": test.link}}
		if message, err := Render(request); err != nil || message.Subject == "" {
			t.Fatalf("Render(%q) = %#v, %v", test.template, message, err)
		}
		request.Variables["invitationLink"] = "https://evil.example/"
		if _, err := Render(request); err == nil {
			t.Fatalf("accepted attacker link for %q", test.template)
		}
	}
}

func TestObsDogInvitationRequiresExactAppFragmentLink(t *testing.T) {
	valid := Request{Template: ObsDogInvitation, Recipient: "person@example.com", Locale: "ko", Variables: map[string]string{"invitationLink": "https://app.obsdog.ai/invitations/accept/#token=abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"}}
	message, err := Render(valid)
	if err != nil {
		t.Fatal(err)
	}
	if message.FromName != "ObsDog" || message.Subject == "" || message.Text == "" {
		t.Fatalf("message = %#v", message)
	}
	for _, link := range []string{
		"https://evil.example/invitations/accept/#token=abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG",
		"https://app.obsdog.ai/invitations/accept/?token=abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG",
		"https://app.obsdog.ai/invitations/accept/#other=abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG",
	} {
		copy := valid
		copy.Variables = map[string]string{"invitationLink": link}
		if _, err := Render(copy); err == nil {
			t.Fatalf("accepted %q", link)
		}
	}
}
