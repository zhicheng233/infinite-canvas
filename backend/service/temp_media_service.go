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

func (s *TempMediaService) SaveMedia(fileHeader *multipart.FileHeader) (*TempMediaUploadResult, error) {
	if fileHeader == nil {
		return nil, fmt.Errorf("媒体文件不能为空")
	}
	if fileHeader.Size <= 0 {
		return nil, fmt.Errorf("媒体文件不能为空")
	}
	mediaType, maxBytes := tempMediaType(fileHeader)
	if mediaType == "" {
		return nil, fmt.Errorf("仅支持图片、视频或音频文件")
	}
	if fileHeader.Size > maxBytes {
		return nil, fmt.Errorf("%s不能超过 %dMB", mediaType, maxBytes/(1024*1024))
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
		switch mediaType {
		case "视频":
			ext = ".mp4"
		case "音频":
			ext = ".mp3"
		default:
			ext = ".png"
		}
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

func tempMediaType(fileHeader *multipart.FileHeader) (string, int64) {
	contentType := strings.ToLower(strings.TrimSpace(fileHeader.Header.Get("Content-Type")))
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	switch {
	case strings.HasPrefix(contentType, "image/") || matchesExtension(ext, ".png", ".jpg", ".jpeg", ".webp", ".gif"):
		return "图片", 30 * 1024 * 1024
	case strings.HasPrefix(contentType, "video/") || matchesExtension(ext, ".mp4", ".mov", ".webm"):
		return "视频", 50 * 1024 * 1024
	case strings.HasPrefix(contentType, "audio/") || matchesExtension(ext, ".mp3", ".wav", ".m4a", ".aac", ".ogg"):
		return "音频", 15 * 1024 * 1024
	default:
		return "", 0
	}
}

func matchesExtension(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
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
