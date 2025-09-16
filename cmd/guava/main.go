package main

import (
	"net/http"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/goiaba-io/guava/internal/config"
	"github.com/goiaba-io/guava/internal/websocket"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = log.Output(os.Stderr)

	ws := websocket.NewServer(&websocket.Options{
		OriginPatterns:  config.Defaults.OriginPatterns,
		IdleTimeout:     config.Defaults.IdleTimeout,
		MaxMessageBytes: config.Defaults.MaxMsgBytes,
	})

	mux := http.NewServeMux()
	mux.Handle("/ws", ws)

	port := config.GetEnv("PORT", config.Defaults.Port)
	addr := ":" + port

	log.Info().
		Str("addr", addr).
		Msg("server starting")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal().Err(err).Msg("server failed")
	}
}
