package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/goiaba-io/guava/internal/ai"
	"github.com/goiaba-io/guava/internal/media"
)

type ControlEnvelope struct {
	Event string `json:"event"`
	ID    string `json:"id,omitempty"`
	Codec string `json:"codec,omitempty"`
}

type WSError struct {
	Error string `json:"error"`
}

type Options struct {
	Logger          *zerolog.Logger
	OriginPatterns  []string
	IdleTimeout     time.Duration
	MaxMessageBytes int64
	MaxTotalBytes   int64
	OutDir          string
}

type Server struct {
	opt *Options
}

func NewServer(opt *Options) *Server {
	if opt.IdleTimeout == 0 {
		opt.IdleTimeout = 60 * time.Second
	}
	if opt.Logger == nil {
		l := log.Logger
		opt.Logger = &l
	}
	if opt.OutDir == "" {
		opt.OutDir = os.TempDir()
	}
	return &Server{opt: opt}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: s.opt.OriginPatterns,
	})
	if err != nil {
		s.opt.Logger.Error().Err(err).Msg("websocket accept failed")
		return
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "bye") }()

	if s.opt.MaxMessageBytes > 0 {
		conn.SetReadLimit(s.opt.MaxMessageBytes)
	}

	s.opt.Logger.Info().Str("remote", r.RemoteAddr).Msg("ws connected")

	var stream *media.ActiveStream

	for {
		ctx, cancel := context.WithTimeout(r.Context(), s.opt.IdleTimeout)
		mt, data, err := conn.Read(ctx)
		cancel()
		if err != nil {
			if websocket.CloseStatus(err) != -1 &&
				!errors.Is(err, context.Canceled) &&
				!errors.Is(err, context.DeadlineExceeded) {
				s.opt.Logger.Warn().Err(err).Msg("ws read ended")
			}
			if stream != nil && stream.W != nil {
				_ = stream.W.Close()
			}
			return
		}

		switch mt {
		case websocket.MessageText:
			var env ControlEnvelope
			if err := json.Unmarshal(data, &env); err != nil {
				s.replyError(r.Context(), conn, "invalid control json")
				continue
			}
			switch strings.ToLower(env.Event) {
			case "start":
				if env.ID == "" || env.Codec == "" {
					s.replyError(r.Context(), conn, "missing id or codec")
					continue
				}
				if !isAllowedCodec(env.Codec) {
					s.replyError(r.Context(), conn, "unsupported codec")
					continue
				}
				if stream != nil && stream.W != nil {
					s.replyError(r.Context(), conn, "stream already active")
					continue
				}

				var w io.WriteCloser
				filename := filepath.Join(s.opt.OutDir, fmt.Sprintf("%s.%s", env.ID, codecToExt(env.Codec)))
				w, err = os.Create(filename)

				if err != nil {
					s.replyError(r.Context(), conn, "failed to create sink")
					s.opt.Logger.Error().Err(err).Msg("create sink failed")
					continue
				}

				stream = &media.ActiveStream{
					ID:       env.ID,
					Codec:    strings.ToLower(env.Codec),
					W:        w,
					Written:  0,
					Filename: filename,
					OpenedAt: time.Now(),
				}

				s.opt.Logger.Info().
					Str("id", stream.ID).
					Str("codec", stream.Codec).
					Msg("stream started")

				_ = wsjson.Write(r.Context(), conn, map[string]string{"status": "started", "id": stream.ID})

			case "end":
				if stream == nil || stream.W == nil {
					s.replyError(r.Context(), conn, "no active stream")
					continue
				}
				_ = stream.W.Close()

				s.opt.Logger.Info().
					Str("id", stream.ID).
					Str("codec", stream.Codec).
					Int64("bytes", stream.Written).
					Float64("duration_sec", time.Since(stream.OpenedAt).Seconds()).
					Msg("stream ended")

				resp, err := ai.TranscribeActiveStream(stream)
				if err != nil {
					s.opt.Logger.Error().Err(err).Msg("transcription failed")
					_ = wsjson.Write(r.Context(), conn, map[string]string{
						"status": "error",
						"error":  err.Error(),
					})
				} else {
					s.opt.Logger.Info().
						Str("id", stream.ID).
						Str("duration", resp.Duration.String()).
						Msg("body: " + string(resp.Body))
					text, _ := resp.JSON["text"].(string)
					_ = wsjson.Write(r.Context(), conn, map[string]string{
						"status":     "success",
						"transcript": text,
						"duration":   resp.Duration.String(),
					})
				}

				stream = nil
			default:
				s.replyError(r.Context(), conn, "unsupported event")
			}

		case websocket.MessageBinary:
			if stream == nil || stream.W == nil {
				s.replyError(r.Context(), conn, "no active stream for this binary frame")
				continue
			}
			n, err := stream.W.Write(data)
			if err != nil {
				s.replyError(r.Context(), conn, "failed to write chunk")
				s.opt.Logger.Error().Err(err).Msg("sink write failed")
				continue
			}
			stream.Written += int64(n)

			if s.opt.MaxTotalBytes > 0 && stream.Written > s.opt.MaxTotalBytes {
				_ = stream.W.Close()
				s.replyError(r.Context(), conn, "stream too large; closed")
				s.opt.Logger.Warn().
					Str("id", stream.ID).
					Int64("bytes", stream.Written).
					Msg("stream exceeded MaxTotalBytes")
				stream = nil
			}

		default:
			s.replyError(r.Context(), conn, "unsupported frame type")
		}
	}
}

func (s *Server) replyError(ctx context.Context, conn *websocket.Conn, msg string) {
	_ = wsjson.Write(ctx, conn, WSError{Error: msg})
}

func isAllowedCodec(codec string) bool {
	switch strings.ToLower(codec) {
	case "opus", "aac", "wav":
		return true
	default:
		return false
	}
}

func codecToExt(codec string) string {
	return strings.ToLower(codec)
}
