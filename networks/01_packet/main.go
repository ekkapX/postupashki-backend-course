package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

var tcpFlagNames = []struct {
	bit  byte
	name string
}{
	{0x01, "FIN"},
	{0x02, "SYN"},
	{0x04, "RST"},
	{0x08, "PSH"},
	{0x10, "ACK"},
	{0x20, "URG"},
}

type ethernet struct {
	dst, src  net.HardwareAddr
	ethertype uint16
	payload   []byte
}

type ipv4 struct {
	version       byte
	ihl           int
	total         int
	id            uint16
	flags         string
	fragOffset    int
	ttl, proto    byte
	src, dst      net.IP
	checksumValid bool
	payload       []byte
}

type tcp struct {
	srcPort, dstPort uint16
	seq, ack         uint32
	offset           int
	flags            string
	window           uint16
	payloadLen       int
}

type udp struct {
	srcPort, dstPort uint16
	length           uint16
	payloadLen       int
}

func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ",")
}

func warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
}

func decodeHex(raw []byte) ([]byte, error) {
	clean := make([]byte, 0, len(raw))
	for _, c := range raw {
		switch c {
		case ' ', '\n', '\r', ':':
			continue
		}
		clean = append(clean, c)
	}
	if len(clean)%2 != 0 {
		return nil, fmt.Errorf("нечётное число hex-цифр: %d", len(clean))
	}
	b, err := hex.DecodeString(string(clean))
	if err != nil {
		return nil, fmt.Errorf("некорректный hex на входе: %w", err)
	}
	return b, nil
}

func parseEthernet(b []byte) (ethernet, error) {
	if len(b) < 14 {
		return ethernet{}, fmt.Errorf("кадр слишком короткий для Ethernet: %d байт, нужно 14", len(b))
	}
	return ethernet{
		dst:       net.HardwareAddr(b[0:6]),
		src:       net.HardwareAddr(b[6:12]),
		ethertype: binary.BigEndian.Uint16(b[12:14]),
		payload:   b[14:],
	}, nil
}

func parseIPv4(b []byte) (ipv4, error) {
	if len(b) < 20 {
		return ipv4{}, fmt.Errorf("слишком короткий IPv4-заголовок: %d байт, нужно минимум 20", len(b))
	}
	var h ipv4
	h.version = b[0] >> 4
	if h.version != 4 {
		return h, fmt.Errorf("ожидалась версия IP 4, получена %d", h.version)
	}
	h.ihl = int(b[0]&0x0f) * 4
	if h.ihl < 20 {
		return h, fmt.Errorf("некорректный IHL: %d байт (меньше 20)", h.ihl)
	}
	if h.ihl > len(b) {
		return h, fmt.Errorf("IHL %d байт больше длины данных (%d байт)", h.ihl, len(b))
	}
	h.total = int(binary.BigEndian.Uint16(b[2:4]))
	if h.total < h.ihl {
		return h, fmt.Errorf("total_length %d меньше длины заголовка %d", h.total, h.ihl)
	}

	h.id = binary.BigEndian.Uint16(b[4:6])
	ff := binary.BigEndian.Uint16(b[6:8])
	var fl []string
	if ff&0x4000 != 0 {
		fl = append(fl, "DF")
	}
	if ff&0x2000 != 0 {
		fl = append(fl, "MF")
	}
	h.flags = joinOrNone(fl)
	h.fragOffset = int(ff&0x1fff) * 8
	h.ttl = b[8]
	h.proto = b[9]
	h.src = net.IP(b[12:16])
	h.dst = net.IP(b[16:20])

	sum := 0
	for i := 0; i < h.ihl; i += 2 {
		sum += int(binary.BigEndian.Uint16(b[i : i+2]))
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	h.checksumValid = sum == 0xffff

	if h.total > len(b) {
		warn("total_length %d больше фактической длины данных %d", h.total, len(b))
		h.payload = b[h.ihl:]
	} else {
		h.payload = b[h.ihl:h.total]
	}
	return h, nil
}

func parseTCP(b []byte) (tcp, error) {
	if len(b) < 20 {
		return tcp{}, fmt.Errorf("слишком короткий TCP-заголовок: %d байт, нужно минимум 20", len(b))
	}
	off := int(b[12]>>4) * 4
	if off < 20 || off > len(b) {
		return tcp{}, fmt.Errorf("некорректный data offset: %d байт (сегмент %d байт)", off, len(b))
	}
	var on []string
	for _, n := range tcpFlagNames {
		if b[13]&n.bit != 0 {
			on = append(on, n.name)
		}
	}
	return tcp{
		srcPort:    binary.BigEndian.Uint16(b[0:2]),
		dstPort:    binary.BigEndian.Uint16(b[2:4]),
		seq:        binary.BigEndian.Uint32(b[4:8]),
		ack:        binary.BigEndian.Uint32(b[8:12]),
		offset:     off,
		flags:      joinOrNone(on),
		window:     binary.BigEndian.Uint16(b[14:16]),
		payloadLen: len(b) - off,
	}, nil
}

func parseUDP(b []byte) (udp, error) {
	if len(b) < 8 {
		return udp{}, fmt.Errorf("слишком короткий UDP-заголовок: %d байт, нужно 8", len(b))
	}
	u := udp{
		srcPort:    binary.BigEndian.Uint16(b[0:2]),
		dstPort:    binary.BigEndian.Uint16(b[2:4]),
		length:     binary.BigEndian.Uint16(b[4:6]),
		payloadLen: len(b) - 8,
	}
	if int(u.length) != len(b) {
		warn("udp.length %d не совпадает с фактической длиной сегмента %d", u.length, len(b))
	}
	return u, nil
}

func run() error {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("чтение stdin: %w", err)
	}
	b, err := decodeHex(raw)
	if err != nil {
		return err
	}

	eth, err := parseEthernet(b)
	if err != nil {
		return err
	}
	fmt.Println("eth.dst", eth.dst)
	fmt.Println("eth.src", eth.src)
	fmt.Printf("eth.ethertype 0x%04x\n", eth.ethertype)
	if eth.ethertype != 0x0800 {
		return nil
	}

	ip, err := parseIPv4(eth.payload)
	if err != nil {
		return err
	}
	fmt.Println("ip.version", ip.version)
	fmt.Println("ip.ihl_bytes", ip.ihl)
	fmt.Println("ip.total_length", ip.total)
	fmt.Printf("ip.id 0x%04x\n", ip.id)
	fmt.Println("ip.flags", ip.flags)
	fmt.Println("ip.frag_offset", ip.fragOffset)
	fmt.Println("ip.ttl", ip.ttl)
	fmt.Println("ip.protocol", ip.proto)
	fmt.Println("ip.src", ip.src)
	fmt.Println("ip.dst", ip.dst)
	fmt.Println("ip.checksum_valid", ip.checksumValid)

	if ip.fragOffset != 0 {
		fmt.Println("payload.length", len(ip.payload))
		return nil
	}

	switch ip.proto {
	case 6:
		t, err := parseTCP(ip.payload)
		if err != nil {
			return err
		}
		fmt.Println("tcp.src_port", t.srcPort)
		fmt.Println("tcp.dst_port", t.dstPort)
		fmt.Println("tcp.seq", t.seq)
		fmt.Println("tcp.ack", t.ack)
		fmt.Println("tcp.data_offset_bytes", t.offset)
		fmt.Println("tcp.flags", t.flags)
		fmt.Println("tcp.window", t.window)
		fmt.Println("payload.length", t.payloadLen)
	case 17:
		u, err := parseUDP(ip.payload)
		if err != nil {
			return err
		}
		fmt.Println("udp.src_port", u.srcPort)
		fmt.Println("udp.dst_port", u.dstPort)
		fmt.Println("udp.length", u.length)
		fmt.Println("payload.length", u.payloadLen)
	default:
		fmt.Println("payload.length", len(ip.payload))
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
