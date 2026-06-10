package pdfgen

import (
	"encoding/base64"
	"errors"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// EncodeQRPNGBase64 renders content as a QR code PNG and returns it
// base64-encoded for direct embedding in an <img src="data:image/png;base64,...">.
func EncodeQRPNGBase64(content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", errors.New("qr content must not be empty")
	}
	png, err := qrcode.Encode(content, qrcode.Medium, 256)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(png), nil
}
