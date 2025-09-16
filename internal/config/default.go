package config

import "time"

type DefaultsConfig struct {
	OriginPatterns  []string
	Port            string
	MaxMsgBytes     int64
	IdleTimeout     time.Duration
	SpeechesBaseURL string
	Model           string
	Language        string
	ResponseFormat  string
	Temperature     string
	Granularities   []string
	Stream          bool
	VADFilter       bool
}

var Defaults = DefaultsConfig{
	OriginPatterns:  []string{"*"},
	Port:            "9000",
	MaxMsgBytes:     1 << 20,
	IdleTimeout:     60 * time.Second,
	SpeechesBaseURL: "http://localhost:8000",
	Model:           "Systran/faster-whisper-tiny",
	Language:        "pt",
	ResponseFormat:  "json",
	Temperature:     "0.3",
	Granularities:   []string{"segment"},
	Stream:          true,
	VADFilter:       true,
}
