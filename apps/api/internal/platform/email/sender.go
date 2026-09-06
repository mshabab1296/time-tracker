package email

import (
	"fmt"
	"net/smtp"
)

type Sender interface {
	Send(to, subject, body string) error
}

type SMTP struct{ Address, From string }

func (sender SMTP) Send(to, subject, body string) error {
	message := []byte(fmt.Sprintf("To: %s\r\nFrom: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n", to, sender.From, subject, body))
	return smtp.SendMail(sender.Address, nil, sender.From, []string{to}, message)
}
