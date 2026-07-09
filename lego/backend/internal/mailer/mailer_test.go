/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mailer

import (
	"context"
	"net/smtp"
	"strings"
	"testing"
)

func TestNewReturnsNilWithoutConfig(t *testing.T) {
	if m := New("", "from@bex.co", "", ""); m != nil {
		t.Error("empty addr must yield a nil mailer (no SMTP configured)")
	}
	if m := New("mail:25", "", "", ""); m != nil {
		t.Error("empty from must yield a nil mailer")
	}
}

func TestSendBuildsRFC5322AndTargetsRecipient(t *testing.T) {
	m := New("mailpit:1025", "bex <noreply@bex.co>", "", "")
	if m == nil {
		t.Fatal("mailer should be configured")
	}
	var gotAddr, gotFrom string
	var gotTo []string
	var gotMsg []byte
	m.sendMail = func(addr string, _ smtp.Auth, from string, to []string, msg []byte) error {
		gotAddr, gotFrom, gotTo, gotMsg = addr, from, to, msg
		return nil
	}
	if err := m.Send(context.Background(), "invitee@example.com", "You're invited", "line one\nline two"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if gotAddr != "mailpit:1025" || gotFrom != "bex <noreply@bex.co>" {
		t.Errorf("envelope: addr=%q from=%q", gotAddr, gotFrom)
	}
	if len(gotTo) != 1 || gotTo[0] != "invitee@example.com" {
		t.Errorf("recipients = %v", gotTo)
	}
	msg := string(gotMsg)
	for _, want := range []string{
		"To: invitee@example.com\r\n",
		"Subject: You're invited\r\n",
		"line one\r\nline two", // body newlines normalized to CRLF
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q in:\n%s", want, msg)
		}
	}
}

func TestPlainAuthSetWhenCredentialsGiven(t *testing.T) {
	if m := New("smtp.sendgrid.net:587", "f@bex.co", "apikey", "secret"); m == nil || m.auth == nil {
		t.Error("username+password must configure PLAIN auth")
	}
	if m := New("mailpit:1025", "f@bex.co", "", ""); m == nil || m.auth != nil {
		t.Error("no username must leave auth unset (Mailpit)")
	}
}
