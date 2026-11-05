package mailpit

import (
	"strings"
	"testing"
)

func TestMIMEBoundsAndSanitization(t *testing.T) {
	raw := "From: x@example.test\r\nSubject: =?UTF-8?B?aGVsbG8=?=\r\nContent-Type: text/plain\r\n\r\nhello\x1b[31m\x00\u202ehidden"
	m, e := Parse("abc", []byte(raw))
	if e != nil || m.Headers["Subject"] != "hello" || strings.ContainsAny(m.Text, "\x1b\x00\u202e") {
		t.Fatalf("%+v %v", m, e)
	}
	raw = "Content-Type: text/html\r\n\r\n<script>alert(1)</script>"
	m, e = Parse("abc", []byte(raw))
	if e != nil || m.Text != "" || m.Notice == "" {
		t.Fatal("HTML rendered", e)
	}
	raw = "Content-Type: text/plain\r\n\r\n" + strings.Repeat("x", 300<<10)
	m, e = Parse("abc", []byte(raw))
	if e != nil || !m.Truncated || len(m.Text) > 256<<10 {
		t.Fatal("text limit", e)
	}
	raw = "Content-Type: text/plain\r\n\r\ninner"
	for i := 0; i < 40; i++ {
		raw = "Content-Type: multipart/mixed; boundary=b\r\n\r\n--b\r\n" + raw + "\r\n--b--\r\n"
	}
	if _, e = Parse("abc", []byte(raw)); e == nil {
		t.Fatal("deep MIME accepted")
	}
	for _, raw := range []string{"not a message", "Content-Type: multipart/mixed\r\n\r\nx", "Content-Transfer-Encoding: base64\r\n\r\ninvalid!!!"} {
		if _, e = Parse("abc", []byte(raw)); e == nil {
			t.Fatal("malformed MIME accepted")
		}
	}
	for _, id := range []string{"../x", "latest", "a/b", "a?x", ""} {
		if ValidID(id) {
			t.Fatal("unsafe ID accepted", id)
		}
	}
}
