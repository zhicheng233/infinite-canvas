package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"infinite-canvas-server/config"
)

type TempMediaService struct {
	dir           string
	publicBaseURL string
}

type TempMediaUploadResult struct {
	URL       string `json:"url"`
	Filename  string `json:"filename"`
	ExpiresAt string `json:"expires_at"`
}

func NewTempMediaService(cfg *config.Config) *TempMediaService {
	return &TempMediaService{
		dir:           cfg.TmpMediaDir,
		publicBaseURL: strings.TrimRight(cfg.PublicBaseURL, "/"),
	}
}

func (s *TempMediaService) SaveImage(fileHeader *multipart.FileHeader) (*TempMediaUploadResult, error) {
	if fileHeader == nil {
		return nil, fmt.Errorf("图片不能为空")
	}
	if fileHeader.Size <= 0 {
		return nil, fmt.Errorf("图片不能为空")
	}
	if fileHeader.Size > 10*1024*1024 {
		return nil, fmt.Errorf("图片不能超过 10MB")
	}

	src, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext == "" {
		ext = ".png"
	}
	filename := fmt.Sprintf("%d-%s%s", time.Now().Unix(), randomToken(12), ext)
	path := filepath.Join(s.dir, filename)
	dst, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer dst.Close()
	if _, err := dst.ReadFrom(src); err != nil {
		return nil, err
	}

	return &TempMediaUploadResult{
		URL:       s.publicURL(filename),
		Filename:  filename,
		ExpiresAt: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	}, nil
}

func (s *TempMediaService) publicURL(filename string) string {
	if s.publicBaseURL != "" {
		return s.publicBaseURL + "/backend-api/media/tmp/" + filename
	}
	return "/backend-api/media/tmp/" + filename
}

func (s *TempMediaService) FilePath(filename string) string {
	return filepath.Join(s.dir, filename)
}

func randomToken(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
