#!/bin/bash

SPEACHES_BASE_URL="http://localhost:8000"

curl -sS -X POST "$SPEACHES_BASE_URL/v1/audio/transcriptions" \
  -F "model=Systran/faster-whisper-small" \
  -F "language=pt" \
  -F "response_format=json" \
  -F "temperature=0.3" \
  -F "timestamp_granularities[]=\"segment\"" \
  -F "vad_filter=true" \
  -F "file=@out.wav"
