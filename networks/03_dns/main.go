package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

var qtypes = map[string]uint16{"A": 1, "NS": 2, "CNAME": 5, "MX": 15, "TXT": 16, "AAAA": 28}

var errBad = errors.New("bad message")

type answer struct {
	line string
	ttl  uint32
}

type entry struct {
	lines []string
	exp   time.Time
}

func buildQuery(id uint16, name string, qt uint16) []byte {
	b := []byte{byte(id >> 8), byte(id), 0x01, 0x00, 0, 1, 0, 0, 0, 0, 0, 0}
	for _, l := range strings.Split(strings.TrimSuffix(name, "."), ".") {
		if l == "" {
			continue
		}
		b = append(b, byte(len(l)))
		b = append(b, l...)
	}
	b = append(b, 0, byte(qt>>8), byte(qt), 0, 1)
	return b
}

// readName читает имя (с учётом сжатия) и возвращает его вместе со смещением
// байта, следующего за именем в исходном месте.
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
			labels = append(labels, string(msg[off+1:off+1+l]))
			off += 1 + l
		}
	}
}

func fmtIPv6(b []byte) string {
	var g [8]uint16
	for i := range g {
		g[i] = binary.BigEndian.Uint16(b[2*i:])
	}
	bestS, bestL := -1, 0
	for i := 0; i < 8; {
		if g[i] != 0 {
			i++
			continue
		}
		j := i
		for j < 8 && g[j] == 0 {
			j++
		}
		if j-i > bestL {
			bestS, bestL = i, j-i
		}
		i = j
	}
	if bestL < 2 {
		bestS = -1
	}
	var sb strings.Builder
	for i := 0; i < 8; i++ {
		if i == bestS {
			sb.WriteString("::")
			i += bestL - 1
			continue
		}
		if sb.Len() > 0 && !strings.HasSuffix(sb.String(), ":") {
			sb.WriteByte(':')
		}
		sb.WriteString(strconv.FormatUint(uint64(g[i]), 16))
	}
	return sb.String()
}

func parse(msg []byte) (int, []answer, error) {
	if len(msg) < 12 {
		return 0, nil, errBad
	}
	rcode := int(msg[3] & 0x0f)
	qd := int(binary.BigEndian.Uint16(msg[4:6]))
	an := int(binary.BigEndian.Uint16(msg[6:8]))
	off := 12
	for i := 0; i < qd; i++ {
		_, next, err := readName(msg, off)
		if err != nil {
			return 0, nil, err
		}
		off = next + 4
	}
	var res []answer
	for i := 0; i < an; i++ {
		_, next, err := readName(msg, off)
		if err != nil {
			return 0, nil, err
		}
		off = next
		if off+10 > len(msg) {
			return 0, nil, errBad
		}
		typ := binary.BigEndian.Uint16(msg[off:])
		ttl := binary.BigEndian.Uint32(msg[off+4:])
		rdlen := int(binary.BigEndian.Uint16(msg[off+8:]))
		rd := off + 10
		end := rd + rdlen
		if end > len(msg) {
			return 0, nil, errBad
		}
		off = end
		var tname, val string
		switch typ {
		case 1:
			if rdlen != 4 {
				continue
			}
			tname = "A"
			val = fmt.Sprintf("%d.%d.%d.%d", msg[rd], msg[rd+1], msg[rd+2], msg[rd+3])
		case 28:
			if rdlen != 16 {
				continue
			}
			tname = "AAAA"
			val = fmtIPv6(msg[rd:end])
		case 2, 5:
			n, _, err := readName(msg, rd)
			if err != nil {
				return 0, nil, err
			}
			tname = "NS"
			if typ == 5 {
				tname = "CNAME"
			}
			val = n
		case 15:
			if rdlen < 3 {
				return 0, nil, errBad
			}
			pref := binary.BigEndian.Uint16(msg[rd:])
			n, _, err := readName(msg, rd+2)
			if err != nil {
				return 0, nil, err
			}
			tname = "MX"
			val = fmt.Sprintf("%d %s", pref, n)
		case 16:
			var sb strings.Builder
			p := rd
			for p < end {
				l := int(msg[p])
				p++
				if p+l > end {
					return 0, nil, errBad
				}
				sb.Write(msg[p : p+l])
				p += l
			}
			tname = "TXT"
			val = sb.String()
		default:
			continue
		}
		res = append(res, answer{fmt.Sprintf("answer %s %s %d", tname, val, ttl), ttl})
	}
	return rcode, res, nil
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

func ask(conn net.Conn, name string, qt uint16) (int, []answer, bool) {
	id := uint16(rand.Intn(65536))
	if _, err := conn.Write(buildQuery(id, name, qt)); err != nil {
		return 0, nil, false
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 65535)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return 0, nil, false
		}
		if n < 12 || binary.BigEndian.Uint16(buf[0:2]) != id {
			continue
		}
		rc, ans, err := parse(buf[:n])
		if err != nil {
			continue
		}
		return rc, ans, true
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: run.sh <addr> <port>")
		os.Exit(2)
	}
	rand.Seed(time.Now().UnixNano())
	conn, err := net.Dial("udp", net.JoinHostPort(os.Args[1], os.Args[2]))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer conn.Close()

	cache := map[string]entry{}
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 2 {
			continue
		}
		name, tstr := f[0], f[1]
		fmt.Println("query", name, tstr)
		qt, ok := qtypes[strings.ToUpper(tstr)]
		if !ok {
			fmt.Println("status FORMERR")
			fmt.Println("end")
			continue
		}
		key := strings.ToLower(strings.TrimSuffix(name, ".")) + "|" + strings.ToUpper(tstr)

		if e, hit := cache[key]; hit {
			if time.Now().Before(e.exp) {
				fmt.Println("status NOERROR")
				for _, l := range e.lines {
					fmt.Println(l)
				}
				fmt.Println("end")
				continue
			}
			delete(cache, key)
		}

		rc, ans, got := ask(conn, name, qt)
		if !got {
			fmt.Println("status TIMEOUT")
			fmt.Println("end")
			os.Exit(1)
		}
		fmt.Println("status", rcodeName(rc))
		for _, a := range ans {
			fmt.Println(a.line)
		}
		fmt.Println("end")

		if rc == 0 && len(ans) > 0 {
			minTTL := ans[0].ttl
			lines := make([]string, len(ans))
			for i, a := range ans {
				lines[i] = a.line
				if a.ttl < minTTL {
					minTTL = a.ttl
				}
			}
			if minTTL > 0 {
				cache[key] = entry{lines, time.Now().Add(time.Duration(minTTL) * time.Second)}
			}
		}
	}
}
