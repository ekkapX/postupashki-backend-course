#!/bin/sh
cd "$(dirname "$0")"
[ -f ./prog.exe ] || go build -o prog.exe main.go
exec ./prog.exe "$@"
