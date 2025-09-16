#!/bin/bash
ffmpeg -f avfoundation -i ":0" -ac 1 -ar 16000 out.wav
