package upload

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"

	"github.com/lzqqdy/marketpulse/internal/config"
)

func newCOSClient(cfg config.UploadConfig) (*cos.Client, error) {
	if !cfg.HasCOS() {
		return nil, nil
	}
	raw := fmt.Sprintf("https://%s.cos.%s.myqcloud.com", strings.TrimSpace(cfg.COSBucket), strings.TrimSpace(cfg.COSRegion))
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("upload cos url: %w", err)
	}
	return cos.NewClient(&cos.BaseURL{BucketURL: u}, &http.Client{
		Timeout: 30 * time.Second,
		Transport: &cos.AuthorizationTransport{
			SecretID:  strings.TrimSpace(cfg.COSSecretID),
			SecretKey: strings.TrimSpace(cfg.COSSecretKey),
		},
	}), nil
}

func (s *Store) saveAvatarCOS(name, contentType string, body io.Reader) (string, error) {
	key := "avatars/" + name
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := s.cos.Object.Put(ctx, key, body, &cos.ObjectPutOptions{
		ACLHeaderOptions: &cos.ACLHeaderOptions{XCosACL: "public-read"},
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType: contentType,
		},
	})
	if err != nil {
		return "", fmt.Errorf("upload cos put: %w", err)
	}
	return fmt.Sprintf("https://%s.cos.%s.myqcloud.com/%s", s.cfg.COSBucket, s.cfg.COSRegion, key), nil
}
