package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/goiaba-io/guava/internal/config"
	"github.com/goiaba-io/guava/internal/media"
)

type TranscriptionResponse struct {
	StatusCode int
	Body       []byte
	JSON       map[string]any
	Duration   time.Duration
}

func TranscribeActiveStream(as *media.ActiveStream) (*TranscriptionResponse, error) {
	if as == nil || as.Filename == "" {
		return nil, fmt.Errorf("invalid ActiveStream: missing Filename")
	}

	f, err := os.Open(as.Filename)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	mw.WriteField("model", config.Defaults.Model)
	mw.WriteField("language", config.Defaults.Language)
	mw.WriteField("response_format", config.Defaults.ResponseFormat)
	mw.WriteField("temperature", config.Defaults.Temperature)
	for _, g := range config.Defaults.Granularities {
		mw.WriteField(`timestamp_granularities[]`, g)
	}
	mw.WriteField("vad_filter", fmt.Sprintf("%t", config.Defaults.VADFilter))

	part, err := mw.CreateFormFile("file", filepath.Base(as.Filename))
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, fmt.Errorf("copy file: %w", err)
	}
	mw.Close()

	endpoint := config.Defaults.SpeechesBaseURL + "/v1/audio/transcriptions"
	req, err := http.NewRequest(http.MethodPost, endpoint, &buf)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Second}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	out := &TranscriptionResponse{
		StatusCode: resp.StatusCode,
		Body:       body,
		Duration:   duration,
	}
	var js map[string]any
	if json.Unmarshal(body, &js) == nil {
		out.JSON = js
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, fmt.Errorf("non-2xx: %d", resp.StatusCode)
	}
	return out, nil
}
