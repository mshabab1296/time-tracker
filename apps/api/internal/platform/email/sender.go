package email

import (
	"context"
	"fmt"
	"net/smtp"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

type Sender interface {
	Send(to, subject, body string) error
}

type SMTP struct{ Address, From string }

func (sender SMTP) Send(to, subject, body string) error {
	message := []byte(fmt.Sprintf("To: %s\r\nFrom: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n", to, sender.From, subject, body))
	return smtp.SendMail(sender.Address, nil, sender.From, []string{to}, message)
}

type sesClient interface {
	SendEmail(context.Context, *sesv2.SendEmailInput, ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

// SES sends production email through the AWS SDK's standard credential chain.
// It supports IAM roles in AWS as well as local AWS profiles for manual testing.
type SES struct {
	Client sesClient
	From   string
}

func NewSES(ctx context.Context, region, from string) (SES, error) {
	configuration, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return SES{}, err
	}
	return SES{Client: sesv2.NewFromConfig(configuration), From: from}, nil
}

func (sender SES) Send(to, subject, body string) error {
	_, err := sender.Client.SendEmail(context.Background(), &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(sender.From),
		Destination:      &types.Destination{ToAddresses: []string{to}},
		Content:          &types.EmailContent{Simple: &types.Message{Subject: &types.Content{Data: aws.String(subject)}, Body: &types.Body{Text: &types.Content{Data: aws.String(body)}}}},
	})
	return err
}
