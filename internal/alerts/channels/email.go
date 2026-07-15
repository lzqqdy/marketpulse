package channels

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"unicode"

	"github.com/lzqqdy/marketpulse/internal/config"
)

// Email sends alert notifications via SMTPS / SMTP.
type Email struct {
	cfg config.SMTPConfig
}

func NewEmail(cfg config.SMTPConfig) *Email {
	return &Email{cfg: cfg}
}

func (e *Email) Configured() bool {
	if e == nil {
		return false
	}
	if strings.TrimSpace(e.cfg.Host) == "" {
		return false
	}
	addr, _, err := e.resolveFrom()
	return err == nil && addr != ""
}

func (e *Email) Send(to, subject, body string) error {
	if !e.Configured() {
		return fmt.Errorf("smtp not configured")
	}
	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("recipient email empty")
	}
	fromAddr, fromHeader, err := e.resolveFrom()
	if err != nil {
		return err
	}
	msg := strings.Join([]string{
		"From: " + fromHeader,
		"To: " + to,
		"Subject: " + encodeHeader(subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: base64",
		"",
		wrapBase64(base64.StdEncoding.EncodeToString([]byte(body))),
	}, "\r\n")
	addr := fmt.Sprintf("%s:%d", e.cfg.Host, e.cfg.Port)
	auth := smtp.PlainAuth("", e.cfg.Username, e.cfg.Password, e.cfg.Host)
	if e.cfg.Port == 465 {
		return sendSMTPS(addr, auth, fromAddr, []string{to}, []byte(msg))
	}
	return smtp.SendMail(addr, auth, fromAddr, []string{to}, []byte(msg))
}

// resolveFrom returns envelope address and RFC5322 From header.
// Accepts bare email, `Name <email>`, or display-name-only (falls back to username).
func (e *Email) resolveFrom() (envelopeAddr, header string, err error) {
	raw := strings.TrimSpace(e.cfg.From)
	user := strings.TrimSpace(e.cfg.Username)
	if raw == "" && user == "" {
		return "", "", fmt.Errorf("smtp from empty")
	}
	if raw == "" {
		return user, user, nil
	}
	if parsed, perr := mail.ParseAddress(raw); perr == nil && parsed.Address != "" {
		if parsed.Name != "" {
			return parsed.Address, formatAddress(parsed.Name, parsed.Address), nil
		}
		return parsed.Address, parsed.Address, nil
	}
	// Display name only (no @) — common misconfig; use username as mailbox.
	if !strings.Contains(raw, "@") {
		if user == "" || !strings.Contains(user, "@") {
			return "", "", fmt.Errorf("smtp from must be an email address")
		}
		return user, formatAddress(raw, user), nil
	}
	return raw, raw, nil
}

func formatAddress(name, address string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return address
	}
	return encodeHeader(name) + " <" + address + ">"
}

func encodeHeader(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	needs := false
	for _, r := range s {
		if r > unicode.MaxASCII || r == '\r' || r == '\n' {
			needs = true
			break
		}
	}
	if !needs {
		return s
	}
	return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(s)) + "?="
}

func wrapBase64(s string) string {
	const line = 76
	if len(s) <= line {
		return s
	}
	var b strings.Builder
	for len(s) > line {
		b.WriteString(s[:line])
		b.WriteString("\r\n")
		s = s[line:]
	}
	b.WriteString(s)
	return b.String()
}

func sendSMTPS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = strings.Split(addr, ":")[0]
	}
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer client.Close()
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}
