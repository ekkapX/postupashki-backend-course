package main

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const usage = "usage: run.sh <url> [--method M] [--max-attempts N] [--idempotency-key K]"

var idempotentMethods = map[string]bool{
	"GET": true, "HEAD": true, "PUT": true, "DELETE": true, "OPTIONS": true, "TRACE": true,
}

var lineBreaks = strings.NewReplacer("\n", " ", "\r", " ")

type config struct {
	url         string
	method      string
	key         string
	maxAttempts int
}

func parseArgs(args []string) (config, error) {
	cfg := config{method: "GET", maxAttempts: 5}
	haveURL := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--method", "--max-attempts", "--idempotency-key":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("флаг %s требует значение", a)
			}
			i++
			v := args[i]
			switch a {
			case "--method":
				if v == "" {
					return cfg, errors.New("--method: пустое значение")
				}
				cfg.method = v
			case "--max-attempts":
				n, err := strconv.Atoi(v)
				if err != nil || n < 1 {
					return cfg, fmt.Errorf("--max-attempts: нужно целое число не меньше 1, получено %q", v)
				}
				cfg.maxAttempts = n
			case "--idempotency-key":
				if v == "" {
					return cfg, errors.New("--idempotency-key: пустое значение")
				}
				cfg.key = v
			}
		default:
			if strings.HasPrefix(a, "-") {
				return cfg, fmt.Errorf("неизвестный флаг %q", a)
			}
			if haveURL {
				return cfg, fmt.Errorf("лишний аргумент %q", a)
			}
			cfg.url, haveURL = a, true
		}
	}
	if !haveURL {
		return cfg, errors.New("не указан URL")
	}
	return cfg, nil
}

func (c config) validate() error {
	u, err := url.Parse(c.url)
	if err != nil {
		return fmt.Errorf("некорректный URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL должен начинаться с http:// или https://, получено %q", c.url)
	}
	if u.Hostname() == "" {
		return fmt.Errorf("в URL нет хоста: %q", c.url)
	}
	if _, err := http.NewRequest(c.method, c.url, nil); err != nil {
		return fmt.Errorf("некорректный запрос: %w", err)
	}
	return nil
}

func newClient() *http.Client {
	return &http.Client{
		Timeout:       5 * time.Second,
		Transport:     &http.Transport{DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func doAttempt(client *http.Client, cfg config) (status, retryAfter int, err error) {
	retryAfter = -1
	req, err := http.NewRequest(cfg.method, cfg.url, nil)
	if err != nil {
		return 0, -1, err
	}
	if cfg.key != "" {
		req.Header.Set("Idempotency-Key", cfg.key)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, -1, err
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return 0, -1, fmt.Errorf("чтение тела: %w", err)
	}
	if v := strings.TrimSpace(resp.Header.Get("Retry-After")); v != "" {
		if n, e := strconv.Atoi(v); e == nil && n >= 0 {
			retryAfter = n
		}
	}
	return resp.StatusCode, retryAfter, nil
}

func backoffLimit(attempt int) int {
	return min(200<<min(attempt, 4), 2000)
}

func backoff(attempt int, rng *rand.Rand) int {
	return rng.Intn(backoffLimit(attempt) + 1)
}

func isRetryableStatus(status int) bool {
	switch status {
	case 429, 500, 502, 503, 504:
		return true
	}
	return false
}

func run(args []string) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}
	if err := cfg.validate(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}

	canRetry := idempotentMethods[cfg.method] || cfg.key != ""
	client := newClient()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for attempt := 1; ; attempt++ {
		status, retryAfter, err := doAttempt(client, cfg)

		retry, success := false, false
		if err != nil {
			fmt.Printf("attempt %d error %s\n", attempt, lineBreaks.Replace(err.Error()))
			retry = true
		} else {
			fmt.Printf("attempt %d status %d\n", attempt, status)
			retry = isRetryableStatus(status)
			success = status >= 200 && status <= 399
		}

		if retry && canRetry && attempt < cfg.maxAttempts {
			ms := backoff(attempt, rng)
			if retryAfter >= 0 {
				ms = retryAfter * 1000
			}
			fmt.Printf("sleep_ms %d\n", ms)
			time.Sleep(time.Duration(ms) * time.Millisecond)
			continue
		}

		if success {
			fmt.Printf("result success attempts %d\n", attempt)
			return 0
		}
		fmt.Printf("result failure attempts %d\n", attempt)
		return 1
	}
}

func main() {
	os.Exit(run(os.Args[1:]))
}
