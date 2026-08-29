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
	request := Request{Template: CerberusInvitation, Recipient: "person@example.com\r\nBcc: attacker@example.com", Locale: "en", Variables: map[string]string{"invitationLink": "https://console.c6s.whitekiwi.link/invitations/accept/#token=x"}}
	if _, err := Render(request); err == nil {
		t.Fatal("accepted recipient injection")
	}
	request.Recipient = "person@example.com"
	request.Variables["unexpected"] = "value"
	if _, err := Render(request); err == nil {
		t.Fatal("accepted unknown template variable")
	}
}
