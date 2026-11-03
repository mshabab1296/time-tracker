package organizations

import (
	"strings"
	"testing"
)

type invitationLinkSender struct {
	to, subject, body string
}

func (sender *invitationLinkSender) Send(to, subject, body string) error {
	sender.to, sender.subject, sender.body = to, subject, body
	return nil
}

func TestInvitationEmailLinksToAcceptanceFlow(t *testing.T) {
	sender := &invitationLinkSender{}
	if err := sendInvitation(sender, "http://localhost:5174/", "invitee@example.com", "Fortune Tech", "opaque-token"); err != nil {
		t.Fatal(err)
	}
	if sender.to != "invitee@example.com" || !strings.Contains(sender.body, "http://localhost:5174/invite?token=opaque-token") {
		t.Fatalf("invitation email not linked to acceptance flow: %+v", sender)
	}
}
