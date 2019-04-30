# Lumberjack -- Golang AWS Lambda Contact Form Handler

This repo contains a short package written in Go to enable a web
contact form. The package is meant to be deployed as an [AWS
Lambda][awslambda] function, making it suitable for a static site. It
uses [Amazon SES][ses] to send an email containing the contact form
information to the site administrator at an address you specify, and
it supports contact forms with binary file uploads, which are sent as
email attachments. I hope this code gets somebody off the hook whose
boss is wondering why it takes so long just to set up a lousy contact
form.

I can't remember every detail of what it takes to get your ducks in a
row with AWS, but the included [Terraform][terraform] configuration
file automates a good chunk of it. If you set things up manually
without Terraform, be sure to enable binary file types under the
resource settings with the [API Gateway][apigateway].
Along with the usual AWS account setup prerequisites, you will need to
customize the Terraform configuration with an AWS region and
the Go source code with
* the same AWS region as the Terraform configuration
* the names of the fields your web form posts
* a sender email address from which the form will appear to originate (validated in advance with SES)
* a destination email address where you want the form sent (validated in advance with SES)
* a confirmation page url (to be shown when sending is successful)
* and an error page url (to be shown when sending fails).

I use this code in production on a small low-traffic personal static
site, but it has some limitations. Server side validation is limited
to a simple anti-spam honeypot  field. If you want to force your users
to give you their real names and email addresses or rent their eyes to
Google, I'll let you figure out that part yourself. There is nothing
about rate limiting or throttling in the Terraform configuration, so
you could get DoS'd by anyone who knows the endpoint An attacker could
easily use up your sandbox email quota but would have to hit it
millions of times to incur significant charges.

## Quick start

Assuming Go and Terraform are installed, your AWS account is set up,
and the files are customized as above:
```console
$ terraform init
$ go get .
$ GOOS=linux GOARCH=amd64 go build -o lumberjack main.go
$ zip -o lumberjack.zip lumberjack
$ terraform apply
```
Then paste the displayed enpoint url into the "action" attribute in the HTML code for your contact form.

## Workflow

Assuming the AWS command line utilities are installed, make quick changes by editing main.go and:
```console
$ GOOS=linux GOARCH=amd64 go build -o lumberjack main.go
$ zip -o lumberjack.zip lumberjack
$ aws lambda update-function-code --region us-west-2 --function-name lumberjack --zip-file fileb://lumberjack.zip
```
with your preferred region substituted for
us-west-2. 

## Teardown

Tear down the whole thing by:
```console
$ terraform destroy
```

[awslambda]:[https://docs.aws.amazon.com/lambda/index.html]
[ses]:[https://docs.aws.amazon.com/ses/]
[apigateway]:[https://docs.aws.amazon.com/apigateway/]
[terraform]:[https://www.terraform.io]

## About the name

It's a quote from a Bob Dylan song, "you have many contacts, among the lumberjacks, ...".
