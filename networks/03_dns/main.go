package main

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"
)

var qtypes = map[string]uint16{"A": 1, "NS": 2, "CNAME": 5, "MX": 15, "TXT": 16, "AAAA": 28}

var (
	errBad     = errors.New("bad message")
	errInput   = errors.New("bad input")
	errTimeout = errors.New("timeout")
)

const classIN = 1

type answer struct {
	typ string
	val string
	ttl uint32
}

type response struct {
	rcode     int
	truncated bool
	answers   []answer
}

type entry struct {
	answers []answer
	exp     time.Time
}

type resolver struct {
	conn  net.Conn
	buf   []byte
	cache map[string]entry
}

func warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
}

func escapeBytes(b []byte, inName bool) string {
	var sb strings.Builder
	for _, c := range b {
		switch {
		case c == '\\':
			sb.WriteString(`\\`)
		case inName && c == '.':
			sb.WriteString(`\.`)
		case c < 0x20 || c > 0x7e || (inName && c == ' '):
			fmt.Fprintf(&sb, `\%03d`, c)
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

func encodeName(name string) ([]byte, error) {
	if name == "." {
		return []byte{0}, nil
	}
	trimmed := strings.TrimSuffix(name, ".")
	if trimmed == "" {
		return nil, errors.New("пустое имя")
	}
	var b []byte
	for _, l := range strings.Split(trimmed, ".") {
		if l == "" {
			return nil, fmt.Errorf("пустая метка в имени %q", name)
		}
		if len(l) > 63 {
			return nil, fmt.Errorf("метка длиннее 63 байт в имени %q", name)
		}
		b = append(b, byte(len(l)))
		b = append(b, l...)
	}
	b = append(b, 0)
	if len(b) > 255 {
		return nil, fmt.Errorf("имя %q длиннее 255 байт", name)
	}
	return b, nil
}

func buildQuery(id uint16, wireName []byte, qt uint16) []byte {
	b := []byte{byte(id >> 8), byte(id), 0x01, 0x00, 0, 1, 0, 0, 0, 0, 0, 0}
	b = append(b, wireName...)
	return append(b, byte(qt>>8), byte(qt), 0, classIN)
}

func readName(msg []byte, off int) (string, int, error) {
	var labels []string
	next := -1
	jumps := 0
	for {
		if off >= len(msg) {
			return "", 0, errBad
		}
		l := int(msg[off])
		switch {
		case l == 0:
			off++
			if next < 0 {
				next = off
			}
			return strings.Join(labels, ".") + ".", next, nil
		case l&0xC0 == 0xC0:
			if off+1 >= len(msg) {
				return "", 0, errBad
			}
			ptr := (l&0x3F)<<8 | int(msg[off+1])
			if next < 0 {
				next = off + 2
			}
			jumps++
			if jumps > 32 {
				return "", 0, errBad
			}
			off = ptr
		case l&0xC0 != 0:
			return "", 0, errBad
		default:
			if off+1+l > len(msg) {
				return "", 0, errBad
			}
			labels = append(labels, escapeBytes(msg[off+1:off+1+l], true))
			off += 1 + l
		}
	}
}

func nameWithin(msg []byte, off, end int) (string, error) {
	n, next, err := readName(msg, off)
	if err != nil {
		return "", err
	}
	if next > end {
		return "", fmt.Errorf("%w: имя выходит за границы rdata", errBad)
	}
	return n, nil
}

func parseRData(msg []byte, typ uint16, rd, end int) (string, string, error) {
	rdlen := end - rd
	switch typ {
	case 1:
		if rdlen != 4 {
			return "", "", fmt.Errorf("A: rdlength %d вместо 4", rdlen)
		}
		return "A", netip.AddrFrom4([4]byte(msg[rd:end])).String(), nil
	case 28:
		if rdlen != 16 {
			return "", "", fmt.Errorf("AAAA: rdlength %d вместо 16", rdlen)
		}
		return "AAAA", netip.AddrFrom16([16]byte(msg[rd:end])).String(), nil
	case 2, 5:
		n, err := nameWithin(msg, rd, end)
		if err != nil {
			return "", "", err
		}
		if typ == 5 {
			return "CNAME", n, nil
		}
		return "NS", n, nil
	case 15:
		if rdlen < 3 {
			return "", "", fmt.Errorf("MX: rdlength %d слишком мал", rdlen)
		}
		pref := binary.BigEndian.Uint16(msg[rd:])
		n, err := nameWithin(msg, rd+2, end)
		if err != nil {
			return "", "", err
		}
		return "MX", fmt.Sprintf("%d %s", pref, n), nil
	case 16:
		var sb strings.Builder
		p := rd
		for p < end {
			l := int(msg[p])
			p++
			if p+l > end {
				return "", "", fmt.Errorf("TXT: строка выходит за границы rdata")
			}
			sb.WriteString(escapeBytes(msg[p:p+l], false))
			p += l
		}
		return "TXT", sb.String(), nil
	}
	return "", "", nil
}

func parse(msg []byte) (response, error) {
	var resp response
	if len(msg) < 12 {
		return resp, errBad
	}
	if msg[2]&0x80 == 0 {
		return resp, fmt.Errorf("%w: QR=0, это не ответ", errBad)
	}
	resp.truncated = msg[2]&0x02 != 0
	resp.rcode = int(msg[3] & 0x0f)
	qd := int(binary.BigEndian.Uint16(msg[4:6]))
	an := int(binary.BigEndian.Uint16(msg[6:8]))

	off := 12
	for i := 0; i < qd; i++ {
		_, next, err := readName(msg, off)
		if err != nil {
			return resp, err
		}
		if next+4 > len(msg) {
			return resp, errBad
		}
		off = next + 4
	}
	for i := 0; i < an; i++ {
		_, next, err := readName(msg, off)
		if err != nil {
			return resp, err
		}
		off = next
		if off+10 > len(msg) {
			return resp, errBad
		}
		typ := binary.BigEndian.Uint16(msg[off:])
		class := binary.BigEndian.Uint16(msg[off+2:])
		ttl := binary.BigEndian.Uint32(msg[off+4:])
		rdlen := int(binary.BigEndian.Uint16(msg[off+8:]))
		rd := off + 10
		end := rd + rdlen
		if end > len(msg) {
			return resp, errBad
		}
		off = end

		if class != classIN {
			warnf("пропущена запись типа %d: класс %d не IN", typ, class)
			continue
		}
		tname, val, err := parseRData(msg, typ, rd, end)
		if err != nil {
			warnf("пропущена битая запись типа %d: %v", typ, err)
			continue
		}
		if tname == "" {
			continue
		}
		if ttl&0x80000000 != 0 {
			ttl = 0
		}
		resp.answers = append(resp.answers, answer{tname, val, ttl})
	}
	return resp, nil
}

func rcodeName(rc int) string {
	switch rc {
	case 0:
		return "NOERROR"
	case 1:
		return "FORMERR"
	case 2:
		return "SERVFAIL"
	case 3:
		return "NXDOMAIN"
	case 5:
		return "REFUSED"
	}
	return fmt.Sprintf("RCODE%d", rc)
}

func newID() (uint16, error) {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(b[:]), nil
}

func lowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

func questionMatches(query, msg []byte) bool {
	q := query[12:]
	if len(msg) < 12+len(q) || binary.BigEndian.Uint16(msg[4:6]) != 1 {
		return false
	}
	for i, c := range q {
		if lowerASCII(c) != lowerASCII(msg[12+i]) {
			return false
		}
	}
	return true
}

func (r *resolver) ask(wireName []byte, qt uint16) (response, error) {
	id, err := newID()
	if err != nil {
		return response{}, fmt.Errorf("генерация ID: %w", err)
	}
	query := buildQuery(id, wireName, qt)
	if _, err := r.conn.Write(query); err != nil {
		return response{}, fmt.Errorf("отправка запроса: %w", err)
	}
	if err := r.conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return response{}, fmt.Errorf("установка дедлайна: %w", err)
	}
	for {
		n, err := r.conn.Read(r.buf)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				return response{}, errTimeout
			}
			return response{}, fmt.Errorf("чтение ответа: %w", err)
		}
		msg := r.buf[:n]
		if n < 12 || binary.BigEndian.Uint16(msg[0:2]) != id {
			continue
		}
		if !questionMatches(query, msg) {
			warnf("ответ с нашим ID, но с другим вопросом, игнорируем")
			continue
		}
		resp, err := parse(msg)
		if err != nil {
			warnf("не удалось разобрать ответ: %v", err)
			continue
		}
		return resp, nil
	}
}

func printBlock(status string, answers []answer) {
	fmt.Println("status", status)
	for _, a := range answers {
		fmt.Printf("answer %s %s %d\n", a.typ, a.val, a.ttl)
	}
	fmt.Println("end")
}

func (r *resolver) store(key string, resp response) {
	if resp.rcode != 0 || resp.truncated || len(resp.answers) == 0 {
		return
	}
	minTTL := resp.answers[0].ttl
	for _, a := range resp.answers {
		if a.ttl < minTTL {
			minTTL = a.ttl
		}
	}
	if minTTL > 0 {
		r.cache[key] = entry{resp.answers, time.Now().Add(time.Duration(minTTL) * time.Second)}
	}
}

func (r *resolver) handle(name, tstr string) error {
	qt, ok := qtypes[strings.ToUpper(tstr)]
	if !ok {
		return fmt.Errorf("%w: неподдерживаемый тип записи %q", errInput, tstr)
	}
	wire, err := encodeName(name)
	if err != nil {
		return fmt.Errorf("%w: %v", errInput, err)
	}

	fmt.Println("query", name, tstr)
	key := strings.ToLower(strings.TrimSuffix(name, ".")) + "|" + strings.ToUpper(tstr)

	if e, hit := r.cache[key]; hit {
		if time.Now().Before(e.exp) {
			printBlock("NOERROR", e.answers)
			return nil
		}
		delete(r.cache, key)
	}

	resp, err := r.ask(wire, qt)
	if errors.Is(err, errTimeout) {
		printBlock("TIMEOUT", nil)
		return err
	}
	if err != nil {
		return err
	}
	if resp.truncated {
		warnf("ответ на %s %s обрезан (TC), результат неполный и не кэшируется", name, tstr)
	}
	printBlock(rcodeName(resp.rcode), resp.answers)
	r.store(key, resp)
	return nil
}

func (r *resolver) run(in io.Reader) int {
	code := 0
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 0 {
			continue
		}
		if len(f) < 2 {
			fmt.Fprintf(os.Stderr, "error: строка %q пропущена: нужны имя и тип\n", sc.Text())
			code = 2
			continue
		}
		err := r.handle(f[0], f[1])
		switch {
		case err == nil:
		case errors.Is(err, errInput):
			fmt.Fprintln(os.Stderr, "error:", err)
			code = 2
		case errors.Is(err, errTimeout):
			return 1
		default:
			fmt.Fprintln(os.Stderr, "error:", err)
			return 2
		}
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error: чтение stdin:", err)
		return 2
	}
	return code
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: run.sh <addr> <port>")
		os.Exit(2)
	}
	conn, err := net.Dial("udp", net.JoinHostPort(os.Args[1], os.Args[2]))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	r := &resolver{conn: conn, buf: make([]byte, 65535), cache: map[string]entry{}}
	code := r.run(os.Stdin)
	conn.Close()
	os.Exit(code)
}
