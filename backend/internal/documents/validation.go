package documents

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const MaxFileBytes = 5 << 20
const MaxBodyBytes = 7 << 20
const MaxDocuments = 20
const MaxMemberBytes = 25 << 20

var errInput = errors.New("invalid document")
var errSize = errors.New("document too large")
var errQuota = errors.New("document quota exceeded")
var errVersion = errors.New("document version conflict")

type uploadInput struct {
	Name          string          `json:"name"`
	Kind          string          `json:"kind"`
	Filename      string          `json:"filename"`
	ContentBase64 string          `json:"contentBase64"`
	ExpiresOn     json.RawMessage `json:"expiresOn"`
}

type validatedUpload struct {
	uploadInput
	content   []byte
	mediaType string
	expiry    *string
}

func safeText(s string, limit int) bool {
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > limit {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}

func validate(in uploadInput) (validatedUpload, error) {
	in.Name, in.Filename = strings.TrimSpace(in.Name), strings.TrimSpace(in.Filename)
	result := validatedUpload{uploadInput: in}
	if in.Name == "" || !safeText(in.Name, 200) || in.Filename == "" || !safeText(in.Filename, 200) || len(in.Filename) > 255 || strings.ContainsAny(in.Filename, "/\\:") || in.Filename == "." || in.Filename == ".." {
		return result, errInput
	}
	switch in.Kind {
	case "medical", "contract", "consent", "other":
	default:
		return result, errInput
	}
	if len(in.ExpiresOn) == 0 || json.Unmarshal(in.ExpiresOn, &result.expiry) != nil {
		return result, errInput
	}
	if result.expiry != nil {
		day, err := time.Parse("2006-01-02", *result.expiry)
		if err != nil || day.Year() < 1 || day.Format("2006-01-02") != *result.expiry {
			return result, errInput
		}
	}
	if len(in.ContentBase64) > base64.StdEncoding.EncodedLen(MaxFileBytes) {
		return result, errSize
	}
	if strings.ContainsAny(in.ContentBase64, "\r\n \t") {
		return result, errInput
	}
	content, err := base64.StdEncoding.Strict().DecodeString(in.ContentBase64)
	if err != nil || len(content) == 0 {
		return result, errInput
	}
	if len(content) > MaxFileBytes {
		return result, errSize
	}
	result.content = content
	result.mediaType = http.DetectContentType(content)
	extension := strings.ToLower(path.Ext(in.Filename))
	switch result.mediaType {
	case "application/pdf":
		if extension != ".pdf" {
			return result, errInput
		}
	case "image/png":
		if extension != ".png" {
			return result, errInput
		}
	case "image/jpeg":
		if extension != ".jpg" && extension != ".jpeg" {
			return result, errInput
		}
	default:
		return result, errInput
	}
	return result, nil
}
