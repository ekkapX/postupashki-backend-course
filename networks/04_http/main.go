package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const (
	httpVersion = "HTTP/1.1"
	crlf        = "\r\n"
	headEnd     = crlf + crlf
)

var (
	errBadStartLine   = errors.New("bad_start_line")
	errBadHeader      = errors.New("bad_header")
	errIncompleteBody = errors.New("incomplete_body")
	errBadChunk       = errors.New("bad_chunk")
)

var parseErrors = []error{errBadStartLine, errBadHeader, errIncompleteBody, errBadChunk}

type header struct {
	name, value string
}

type message struct {
	isRequest bool
	method    string
	target    string
	version   string
	status    string
	reason    string
	headers   []header
	body      []byte
}

func parse(data []byte) (*message, error) {
	line, rest, ok := bytes.Cut(data, []byte(crlf))
	if !ok {
		return nil, fmt.Errorf("%w: строка начала не завершена CRLF", errBadStartLine)
	}

	m := &message{}
	if err := m.parseStartLine(string(line)); err != nil {
		return nil, err
	}

	head, rest, err := splitHead(rest)
	if err != nil {
		return nil, err
	}
	if m.headers, err = parseHeaders(head); err != nil {
		return nil, err
	}
	if m.body, err = m.readBody(rest); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *message) parseStartLine(line string) error {
	if strings.HasPrefix(line, "HTTP/") {
		return m.parseStatusLine(line)
	}
	return m.parseRequestLine(line)
}

func (m *message) parseRequestLine(line string) error {
	parts := strings.Split(line, " ")
	if len(parts) != 3 {
		return fmt.Errorf("%w: ожидалось «метод цель версия», получено %q", errBadStartLine, line)
	}
	method, target, version := parts[0], parts[1], parts[2]
	if !isToken(method) || target == "" || hasControl(target) || version != httpVersion {
		return fmt.Errorf("%w: некорректная строка запроса %q", errBadStartLine, line)
	}
	m.isRequest, m.method, m.target, m.version = true, method, target, version
	return nil
}

func (m *message) parseStatusLine(line string) error {
	version, rest, _ := strings.Cut(line, " ")
	status, reason, _ := strings.Cut(rest, " ")
	if version != httpVersion || len(status) != 3 || !allBytes(status, isDigit) {
		return fmt.Errorf("%w: некорректная строка состояния %q", errBadStartLine, line)
	}
	m.version, m.status, m.reason = version, status, reason
	return nil
}

func splitHead(rest []byte) (head, body []byte, err error) {
	if bytes.HasPrefix(rest, []byte(crlf)) {
		return nil, rest[len(crlf):], nil
	}
	head, body, ok := bytes.Cut(rest, []byte(headEnd))
	if !ok {
		return nil, nil, fmt.Errorf("%w: заголовок не завершён пустой строкой", errBadHeader)
	}
	return head, body, nil
}

func parseHeaders(head []byte) ([]header, error) {
	if len(head) == 0 {
		return nil, nil
	}
	lines := strings.Split(string(head), crlf)
	headers := make([]header, 0, len(lines))
	for _, line := range lines {
		name, value, ok := strings.Cut(line, ":")
		if !ok || !isToken(name) {
			return nil, fmt.Errorf("%w: некорректное поле %q", errBadHeader, line)
		}
		headers = append(headers, header{
			name:  strings.ToLower(name),
			value: strings.Trim(value, " \t"),
		})
	}
	return headers, nil
}

func (m *message) values(name string) []string {
	var out []string
	for _, h := range m.headers {
		if h.name == name {
			out = append(out, h.value)
		}
	}
	return out
}

func (m *message) isChunked() bool {
	last := ""
	for _, v := range m.values("transfer-encoding") {
		for _, coding := range strings.Split(v, ",") {
			last = strings.ToLower(strings.Trim(coding, " \t"))
		}
	}
	return last == "chunked"
}

func (m *message) readBody(rest []byte) ([]byte, error) {
	if m.isChunked() {
		return readChunked(rest)
	}
	cl := m.values("content-length")
	if len(cl) == 0 {
		return nil, nil
	}
	n, err := parseLength(cl[0])
	if err != nil {
		return nil, err
	}
	if n > uint64(len(rest)) {
		return nil, fmt.Errorf("%w: ожидалось %d байт, есть %d", errIncompleteBody, n, len(rest))
	}
	return rest[:n], nil
}

func parseLength(s string) (uint64, error) {
	if s == "" || !allBytes(s, isDigit) {
		return 0, fmt.Errorf("%w: Content-Length %q не число", errBadHeader, s)
	}
	n, err := strconv.ParseUint(s, 10, 63)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0, fmt.Errorf("%w: Content-Length %q: %v", errBadHeader, s, err)
	}
	return n, nil
}

func readChunked(data []byte) ([]byte, error) {
	var body []byte
	for {
		line, rest, ok := bytes.Cut(data, []byte(crlf))
		if !ok {
			return nil, fmt.Errorf("%w: нет строки длины или части нулевой длины", errBadChunk)
		}
		size, err := parseChunkSize(string(line))
		if err != nil {
			return nil, err
		}
		if size == 0 {
			return body, nil // концевик игнорируется
		}
		if size > uint64(len(rest)) || !bytes.HasPrefix(rest[size:], []byte(crlf)) {
			return nil, fmt.Errorf("%w: часть короче %d байт или не завершена CRLF", errBadChunk, size)
		}
		body = append(body, rest[:size]...)
		data = rest[size+uint64(len(crlf)):]
	}
}

func parseChunkSize(line string) (uint64, error) {
	s, _, _ := strings.Cut(line, ";")
	s = strings.Trim(s, " \t")
	if s == "" || !allBytes(s, isHex) {
		return 0, fmt.Errorf("%w: длина %q не hex-число", errBadChunk, s)
	}
	n, err := strconv.ParseUint(s, 16, 63)
	if err != nil {
		return 0, fmt.Errorf("%w: длина %q: %v", errBadChunk, s, err)
	}
	return n, nil
}

func (m *message) render() string {
	var b strings.Builder
	line := func(key, value string) { fmt.Fprintf(&b, "%s %s\n", key, value) }

	if m.isRequest {
		line("type", "request")
		line("method", m.method)
		line("target", m.target)
		line("version", m.version)
	} else {
		line("type", "response")
		line("version", m.version)
		line("status", m.status)
		line("reason", m.reason)
	}
	for _, h := range m.headers {
		line("header", h.name+" "+h.value)
	}
	line("body.length", strconv.Itoa(len(m.body)))
	if len(m.body) > 0 && isPrintable(m.body) {
		line("body.text", string(m.body))
	}
	return b.String()
}

func reasonOf(err error) string {
	for _, e := range parseErrors {
		if errors.Is(err, e) {
			return e.Error()
		}
	}
	return err.Error()
}

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

func isHex(c byte) bool {
	return isDigit(c) || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

func isTchar(c byte) bool {
	switch {
	case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', isDigit(c):
		return true
	}
	return strings.IndexByte("!#$%&'*+-.^_`|~", c) >= 0
}

func isToken(s string) bool { return s != "" && allBytes(s, isTchar) }

func hasControl(s string) bool {
	return !allBytes(s, func(c byte) bool { return c > 0x20 && c != 0x7F })
}

func isPrintable(b []byte) bool {
	for _, c := range b {
		if c < 0x20 || c > 0x7E {
			return false
		}
	}
	return true
}

func allBytes(s string, ok func(byte) bool) bool {
	for i := 0; i < len(s); i++ {
		if !ok(s[i]) {
			return false
		}
	}
	return true
}

func run(in io.Reader, out, errOut io.Writer) int {
	data, err := io.ReadAll(in)
	if err != nil {
		fmt.Fprintln(errOut, "error:", err)
		return 2
	}
	m, err := parse(data)
	if err != nil {
		fmt.Fprintln(out, "error", reasonOf(err))
		fmt.Fprintln(errOut, "error:", err)
		return 1
	}
	fmt.Fprint(out, m.render())
	return 0
}

func main() {
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr))
}
