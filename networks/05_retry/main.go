package main

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	args := os.Args[1:]
	url, method, key := "", "GET", ""
	maxAtt := 5
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--method":
			if i+1 < len(args) {
				i++
				method = strings.ToUpper(args[i])
			}
		case "--max-attempts":
			if i+1 < len(args) {
				i++
				if n, err := strconv.Atoi(args[i]); err == nil {
					maxAtt = n
				}
			}
		case "--idempotency-key":
			if i+1 < len(args) {
				i++
				key = args[i]
			}
		default:
			if url == "" {
				url = args[i]
			}
		}
	}
	if url == "" {
		fmt.Fprintln(os.Stderr, "usage: run.sh <url> [--method M] [--max-attempts N] [--idempotency-key K]")
		os.Exit(2)
	}

	idempotent := map[string]bool{"GET": true, "HEAD": true, "PUT": true, "DELETE": true, "OPTIONS": true, "TRACE": true}
	canRetry := idempotent[method] || key != ""

	client := &http.Client{
		Timeout:       5 * time.Second,
		Transport:     &http.Transport{DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for attempt := 1; ; attempt++ {
		status := 0
		retryAfter := -1
		var reqErr error

		req, err := http.NewRequest(method, url, nil)
		if err == nil {
			if key != "" {
				req.Header.Set("Idempotency-Key", key)
			}
			var resp *http.Response
			resp, err = client.Do(req)
			if err == nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				status = resp.StatusCode
				if v := strings.TrimSpace(resp.Header.Get("Retry-After")); v != "" {
					if n, e := strconv.Atoi(v); e == nil && n >= 0 {
						retryAfter = n
					}
				}
			}
		}
		reqErr = err

		retry := false
		success := false
		if reqErr != nil {
			msg := strings.NewReplacer("\n", " ", "\r", " ").Replace(reqErr.Error())
			fmt.Printf("attempt %d error %s\n", attempt, msg)
			retry = true
		} else {
			fmt.Printf("attempt %d status %d\n", attempt, status)
			switch status {
			case 429, 500, 502, 503, 504:
				retry = true
			}
			success = status >= 200 && status <= 399
		}

		if retry && canRetry && attempt < maxAtt {
			var ms int
			if retryAfter >= 0 && reqErr == nil {
				ms = retryAfter * 1000
			} else {
				limit := 200 << uint(attempt)
				if limit > 2000 || limit <= 0 {
					limit = 2000
				}
				ms = rng.Intn(limit + 1)
			}
			fmt.Printf("sleep_ms %d\n", ms)
			time.Sleep(time.Duration(ms) * time.Millisecond)
			continue
		}

		if success {
			fmt.Printf("result success attempts %d\n", attempt)
			os.Exit(0)
		}
		fmt.Printf("result failure attempts %d\n", attempt)
		os.Exit(1)
	}
}
