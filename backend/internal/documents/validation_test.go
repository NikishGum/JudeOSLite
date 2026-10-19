package documents

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

var pdf = []byte("%PDF-1.7\nSynthetic private bytes\n%%EOF\n")
var png = []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13}
var jpeg = []byte{255, 216, 255, 224, 0, 16, 'J', 'F', 'I', 'F', 0}

func input(content []byte, filename string) uploadInput {
	return uploadInput{Name: "Synthetic medical title", Kind: "medical", Filename: filename, ContentBase64: base64.StdEncoding.EncodeToString(content), ExpiresOn: json.RawMessage(`null`)}
}
func TestValidation(t *testing.T) {
	for _, v := range []struct {
		data       []byte
		name, mime string
	}{{pdf, "справка 2026.pdf", "application/pdf"}, {png, "image.PNG", "image/png"}, {jpeg, "image.jpeg", "image/jpeg"}} {
		got, err := validate(input(v.data, v.name))
		if err != nil || got.mediaType != v.mime || !bytes.Equal(got.content, v.data) {
			t.Fatalf("valid file: %v", err)
		}
	}
	cases := map[string]func(*uploadInput){
		"empty":        func(i *uploadInput) { i.ContentBase64 = "" },
		"base64":       func(i *uploadInput) { i.ContentBase64 = "%%%%" },
		"noncanonical": func(i *uploadInput) { i.ContentBase64 = "Zh==" },
		"newline":      func(i *uploadInput) { i.ContentBase64 += "\n" },
		"html": func(i *uploadInput) {
			i.ContentBase64 = base64.StdEncoding.EncodeToString([]byte("<!DOCTYPE html><html>"))
		},
		"svg":              func(i *uploadInput) { i.ContentBase64 = base64.StdEncoding.EncodeToString([]byte("<svg></svg>")) },
		"mismatch":         func(i *uploadInput) { i.Filename = "wrong.png" },
		"double extension": func(i *uploadInput) { i.Filename = "file.pdf.html" },
		"path":             func(i *uploadInput) { i.Filename = "../file.pdf" },
		"windows":          func(i *uploadInput) { i.Filename = "C:\\file.pdf" },
		"header":           func(i *uploadInput) { i.Filename = "a\r\nX-Foo: bar.pdf" },
		"bidi":             func(i *uploadInput) { i.Filename = "file\u202e.pdf" },
		"no name":          func(i *uploadInput) { i.Name = " " },
		"kind":             func(i *uploadInput) { i.Kind = "secret" },
		"missing expiry":   func(i *uploadInput) { i.ExpiresOn = nil },
		"date":             func(i *uploadInput) { i.ExpiresOn = json.RawMessage(`"2026-02-30"`) },
		"zero year":        func(i *uploadInput) { i.ExpiresOn = json.RawMessage(`"0000-01-01"`) },
		"long filename":    func(i *uploadInput) { i.Filename = strings.Repeat("я", 127) + ".pdf" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			in := input(pdf, "test.pdf")
			change(&in)
			if _, err := validate(in); err != errInput {
				t.Fatalf("wanted invalid: %v", err)
			}
		})
	}
	data := append([]byte{}, pdf...)
	data = append(data, bytes.Repeat([]byte{'x'}, MaxFileBytes-len(data))...)
	if got, err := validate(input(data, "max.pdf")); err != nil || len(got.content) != MaxFileBytes {
		t.Fatalf("5MiB boundary: %v", err)
	}
	if _, err := validate(input(append(data, 'x'), "max.pdf")); err != errSize {
		t.Fatalf("oversize: %v", err)
	}
	in := input(pdf, "expiry.pdf")
	in.ExpiresOn = json.RawMessage(`"2028-02-29"`)
	if _, err := validate(in); err != nil {
		t.Fatal(err)
	}
}
func TestBodyLimitAndJSON(t *testing.T) {
	for _, tc := range []struct {
		body, ct string
		limit    int64
		status   int
	}{{`{"x":"` + strings.Repeat("x", 200) + `"}`, "application/json", 100, 413}, {`{"name":"x"} {}`, "application/json", 100, 400}, {`{"unknown":1}`, "application/json", 100, 400}, {`{}`, "text/plain", 100, 400}} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", tc.ct)
		if decode(w, r, &uploadInput{}, tc.limit) || w.Code != tc.status {
			t.Fatalf("decode %d: %s", w.Code, w.Body.String())
		}
	}
}
