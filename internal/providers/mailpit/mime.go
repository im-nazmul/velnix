package mailpit

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strconv"
	"strings"

	"velnix/internal/domain"
)

type Attachment struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
	Bytes       []byte `json:"-"`
}
type Message struct {
	ID          string            `json:"id"`
	Headers     map[string]string `json:"headers"`
	Text        string            `json:"text"`
	Notice      string            `json:"notice,omitempty"`
	Attachments []Attachment      `json:"attachments"`
	Truncated   bool              `json:"truncated"`
}

func Parse(id string, raw []byte) (Message, error) {
	out := Message{ID: id, Headers: map[string]string{}, Attachments: []Attachment{}}
	if len(raw) > MaxRaw {
		return out, domain.E(domain.Invalid, "mail message exceeds raw limit")
	}
	m, e := mail.ReadMessage(bytes.NewReader(raw))
	if e != nil {
		return out, domain.E(domain.Invalid, "malformed mail headers")
	}
	decoder := &mime.WordDecoder{}
	for _, key := range []string{"From", "To", "Cc", "Bcc", "Return-Path", "Reply-To", "Subject", "Date", "Message-ID"} {
		v := m.Header.Get(key)
		if len(v) > 8192 {
			return out, domain.E(domain.Invalid, "mail header exceeds display limit")
		}
		if decoded, e := decoder.DecodeHeader(v); e == nil {
			v = decoded
		}
		out.Headers[key] = Safe(v)
	}
	parts, decoded, textBytes := 0, 0, 0
	var walk func(mapHeader, io.Reader, int) error
	walk = func(h mapHeader, r io.Reader, depth int) error {
		parts++
		if parts > 128 || depth > 32 {
			return domain.E(domain.Invalid, "MIME nesting/part limit exceeded")
		}
		typ, params, e := mime.ParseMediaType(h.Get("Content-Type"))
		if e != nil {
			if h.Get("Content-Type") == "" {
				typ = "text/plain"
				params = map[string]string{}
			} else {
				return domain.E(domain.Invalid, "malformed MIME content type")
			}
		}
		if strings.HasPrefix(typ, "multipart/") {
			boundary := params["boundary"]
			if boundary == "" || len(boundary) > 200 {
				return domain.E(domain.Invalid, "invalid MIME boundary")
			}
			reader := multipart.NewReader(r, boundary)
			for {
				part, e := reader.NextRawPart()
				if e == io.EOF {
					return nil
				}
				if e != nil {
					return domain.E(domain.Invalid, "malformed MIME multipart")
				}
				err := walk(header(part.Header), part, depth+1)
				part.Close()
				if err != nil {
					return err
				}
			}
		}
		switch strings.ToLower(h.Get("Content-Transfer-Encoding")) {
		case "base64":
			r = base64.NewDecoder(base64.StdEncoding, r)
		case "quoted-printable":
			r = quotedprintable.NewReader(r)
		case "", "7bit", "8bit", "binary":
		default:
			return domain.E(domain.Invalid, "unsupported MIME transfer encoding")
		}
		b, e := io.ReadAll(io.LimitReader(r, int64(MaxRaw-decoded+1)))
		if e != nil {
			return domain.E(domain.Invalid, "invalid MIME body encoding")
		}
		decoded += len(b)
		if decoded > MaxRaw {
			return domain.E(domain.Invalid, "decoded MIME exceeds limit")
		}
		disposition, dispParams, _ := mime.ParseMediaType(h.Get("Content-Disposition"))
		name := dispParams["filename"]
		if name == "" {
			name = params["name"]
		}
		if len(name) > 4096 {
			return domain.E(domain.Invalid, "attachment name exceeds limit")
		}
		if name != "" || disposition == "attachment" || (typ != "text/plain" && typ != "text/html") {
			out.Attachments = append(out.Attachments, Attachment{strconv.Itoa(len(out.Attachments) + 1), Safe(name), Safe(typ), len(b), b})
			return nil
		}
		if typ == "text/plain" {
			const maxText = 256 << 10
			available := maxText - textBytes
			if len(b) > available {
				b = b[:available]
				out.Truncated = true
			}
			textBytes += len(b)
			if out.Text != "" {
				out.Text += "\n"
			}
			out.Text += Safe(string(b))
			charset := strings.ToLower(params["charset"])
			if charset != "" && charset != "utf-8" && charset != "us-ascii" {
				out.Notice = "Non-UTF-8 text may contain replacement characters."
			}
		}
		return nil
	}
	if e = walk(header(m.Header), m.Body, 0); e != nil {
		return out, e
	}
	if out.Text == "" {
		out.Notice = "No plain-text body available; HTML preview is unavailable in this phase."
	}
	return out, nil
}

type mapHeader interface{ Get(string) string }
type header map[string][]string

func (h header) Get(key string) string { return mail.Header(h).Get(key) }
func (m Message) Attachment(id string) (Attachment, error) {
	for _, a := range m.Attachments {
		if a.ID == id {
			return a, nil
		}
	}
	return Attachment{}, domain.E(domain.Unavailable, "attachment %s unavailable", fmt.Sprintf("%q", Safe(id)))
}
