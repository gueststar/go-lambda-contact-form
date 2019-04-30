//    aws lambda function handler for web page contact forms with binary attachments
//    copyright (c) 2019 Dennis Furey

//    This program is free software: you can redistribute it and/or modify
//    it under the terms of the GNU General Public License as published by
//    the Free Software Foundation, either version 3 of the License, or
//    (at your option) any later version.
//
//    This program is distributed in the hope that it will be useful,
//    but WITHOUT ANY WARRANTY; without even the implied warranty of
//    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//    GNU General Public License for more details.
//
//    You should have received a copy of the GNU General Public License
//    along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"io"
	"fmt"
	"mime"
	"bytes"
	"errors"
	"strings"
	"net/textproto"
	"mime/multipart"
	"encoding/base64"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
)

const (
	success_page = "http://www.example.com/thankyou.html"  // redirect here when the contact form submission succeeds
	failure_page = "http://www.example.com/problem.html"   // redirect here when the contact form submission fails
	sender       = "my_contact_form@example.com"           // form mailed from here; must be validated in advance with SES
	recipient    = "my_personal_email@my_provider.com"     // sent to here; must be validated in advance with SES
	subject      = "contact me"
	charset      = "UTF-8"
	region       = "us-west-2"
)


func confirmation(url string) (events.APIGatewayProxyResponse, error) {

	// Make the user's browser load the page at the given url and exit
	// the handler.

	res := events.APIGatewayProxyResponse{
		StatusCode: 301,                                     // not 200 or else
		Headers:    map[string]string{"Location": url},
	}
	return res, nil
}




func form_fields (req events.APIGatewayProxyRequest) (*multipart.Form, error) {

	// Return the contact form fields from the base64 encoded request
	// body. Maintainers of this code should check header field
	// identifier capitalizations if it stops working after an API
	// update.

	decoded, err := base64.StdEncoding.DecodeString(req.Body)
	if err != nil {
		return nil, err
	}
	mediatype, parts, err := mime.ParseMediaType(req.Headers["content-type"])  // must be lower case content-type or else
	if err != nil {
		return nil, err
	}
	if ! strings.HasPrefix(mediatype, "multipart/") {                          // probably better be lower case here too
		return nil, err
	}
	mr := multipart.NewReader(bytes.NewReader (decoded), parts["boundary"])    // lower case here too
	return mr.ReadForm (16777216)                                              // 16 mb of memory, but 10 are enough
}





func email_body (form *multipart.Form, message *bytes.Buffer) (*multipart.Writer, error) {

	// Initialize and return a multipart message writer associated with
	// the given buffer after writing the first part of it, which will
	// be made to contain the text portion of the email triggered by
	// the contact form submission. The contact form depends on the web
	// front end and is assumed to have a honeypot field and three
	// useful fields named as shown.

	mw := multipart.NewWriter (message)
	h := textproto.MIMEHeader{
		"Content-Type": {"text/plain; charset=utf-8"},
		"Content-Transfer-Encoding": {"quoted-printable"},
	}
	body, err := mw.CreatePart (h)
	if err != nil {
		return mw, err
	}
	field := strings.Join(form.Value["office"],"\n")  // hidden field that should always be empty unless filled by a bot,
	if field != "" {                                               // plausibly named with display:none buried in the CSS
		err = errors.New ("spambot attack suspected")
		return mw, err
	}
	field = strings.Join(form.Value["name"],"\n")
	if field == "" {
		field = "(withheld)"
	}
	_, err = fmt.Fprintf (body, "name:  %s\n\n", field)
	if err != nil {
		return mw, err
	}
	field = strings.Join(form.Value["email"],"\n")
	if field == "" {
		field = "(withheld)"
	}
	_, err = fmt.Fprintf (body, "email: %s\n\n", field)
	if err != nil {
		return mw, err
	}
	field = strings.Join(form.Value["message"],"\n")
	if field == "" {
		field = "(withheld)"
	}
	_, err = fmt.Fprintf (body, "message:\n\n%s", field)
	return mw, err
}





func unattachable (mw *multipart.Writer, fileheader *multipart.FileHeader) error {

	// Load the content of an attachable file from the fileheader into
	// the next part of the multipart message context mw in base64
	// encoded format.

	mimetype := ""
	if strings.Contains (fileheader.Filename, ".") {
		fields := strings.Split (fileheader.Filename, ".")
		mimetype = mime.TypeByExtension("." + fields[len(fields) - 1])
	}
	if mimetype == "" {
		mimetype = "application/octet-stream"
	}
	h := textproto.MIMEHeader{
		"Content-Type": {mimetype},
		"Content-Transfer-Encoding": {"base64"},
		"Content-Disposition": {"attachment; filename=\"" + fileheader.Filename + "\""},
	}
	attachment_src, err := fileheader.Open ()
	if err != nil {
		return err
	}
	attachment_dest, err := mw.CreatePart (h)
	if err != nil {
		return err
	}
	encoder := base64.NewEncoder (base64.StdEncoding, attachment_dest)
	_, err = io.Copy (encoder, attachment_src)
	if err != nil {
		return err
	}
	return encoder.Close ()
}




func header_of (boundary string) []byte {

	// Return the email header, which depends on the multipart message
	// boundary. The subject has to come last or else.

	var header bytes.Buffer

	header.WriteString("MIME-Version: 1.0\n")
	header.WriteString("Content-Disposition: inline\n")
	header.WriteString("Content-Type: multipart/mixed; boundary=\"" + boundary + "\"\n")
	header.WriteString("From: " + sender + "\n")
	header.WriteString("To: " + recipient + "\n")
	header.WriteString("Subject: " + subject +"\n\n")
	return header.Bytes()
}





func unsendable (header []byte, message []byte) error {

	// Put the header and the message together and mail them with the
	// SES raw message API.

	params := &ses.SendRawEmailInput{RawMessage: &ses.RawMessage{ Data: bytes.Join ([][]byte{header,message}, []byte{})}}
	sess, err := session.NewSession(&aws.Config{	Region:aws.String(region)}	)
	if err == nil {
		svc := ses.New(sess)
		_, err = svc.SendRawEmail(params)
	}
	return err
}




func handler(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	// Get the form fields from the request, email them, and show a
	// confirmation.

	var message bytes.Buffer

	form, err := form_fields (req)
	if err != nil {
		return confirmation (failure_page)
	}
	mw, err := email_body (form, &message)
	for _, fileheader := range form.File["attachment"] {
		if (err == nil) && (fileheader.Filename != "") {
			err = unattachable (mw, fileheader)
		}
	}
	if (err != nil) || (mw.Close () != nil) {
		return confirmation (failure_page)
	}
	if unsendable (header_of (mw.Boundary ()), message.Bytes ()) != nil {
		return confirmation (failure_page)
	}
	return confirmation (success_page)
}




func main() {
	lambda.Start(handler)
}
