package main

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

func be16(b []byte) int { return int(b[0])<<8 | int(b[1]) }
func be32(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

func mac(b []byte) string {
	p := make([]string, 6)
	for i := range 6 {
		p[i] = fmt.Sprintf("%02x", b[i])
	}
	return strings.Join(p, ":")
}

func ipStr(b []byte) string {
	return fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3])
}

func main() {
	raw, _ := io.ReadAll(os.Stdin)

	// оставляем только hex-цифры
	var sb strings.Builder
	for _, c := range string(raw) {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			sb.WriteRune(c)
		}
	}
	s := sb.String()
	if len(s)%2 == 1 {
		s = s[:len(s)-1]
	}
	b, _ := hex.DecodeString(s)

	// Ethernet
	if len(b) < 14 {
		return
	}
	fmt.Println("eth.dst", mac(b[0:6]))
	fmt.Println("eth.src", mac(b[6:12]))
	etype := be16(b[12:14])
	fmt.Printf("eth.ethertype 0x%04x\n", etype)
	if etype != 0x0800 {
		return
	}

	// IPv4
	ip := b[14:]
	if len(ip) < 20 {
		return
	}
	ihl := int(ip[0]&0x0f) * 4
	total := be16(ip[2:4])
	ff := be16(ip[6:8])

	var fl []string
	if ff&0x4000 != 0 {
		fl = append(fl, "DF")
	}
	if ff&0x2000 != 0 {
		fl = append(fl, "MF")
	}
	flags := "none"
	if len(fl) > 0 {
		flags = strings.Join(fl, ",")
	}

	// контрольная сумма
	valid := false
	if ihl >= 20 && ihl <= len(ip) {
		sum := 0
		for i := 0; i+1 < ihl; i += 2 {
			sum += be16(ip[i : i+2])
		}
		for sum>>16 != 0 {
			sum = (sum & 0xffff) + (sum >> 16)
		}
		valid = sum == 0xffff
	}

	proto := int(ip[9])
	fmt.Println("ip.version", ip[0]>>4)
	fmt.Println("ip.ihl_bytes", ihl)
	fmt.Println("ip.total_length", total)
	fmt.Printf("ip.id 0x%04x\n", be16(ip[4:6]))
	fmt.Println("ip.flags", flags)
	fmt.Println("ip.frag_offset", (ff&0x1fff)*8)
	fmt.Println("ip.ttl", ip[8])
	fmt.Println("ip.protocol", proto)
	fmt.Println("ip.src", ipStr(ip[12:16]))
	fmt.Println("ip.dst", ipStr(ip[16:20]))
	fmt.Println("ip.checksum_valid", valid)

	// обрезаем дополнение
	if total <= len(ip) {
		ip = ip[:total]
	}
	if ihl > len(ip) {
		return
	}
	tr := ip[ihl:]

	switch {
	case proto == 6 && len(tr) >= 20:
		off := int(tr[12]>>4) * 4
		f := tr[13]
		names := []struct {
			bit  byte
			name string
		}{{0x01, "FIN"}, {0x02, "SYN"}, {0x04, "RST"}, {0x08, "PSH"}, {0x10, "ACK"}, {0x20, "URG"}}
		var on []string
		for _, n := range names {
			if f&n.bit != 0 {
				on = append(on, n.name)
			}
		}
		tf := "none"
		if len(on) > 0 {
			tf = strings.Join(on, ",")
		}
		fmt.Println("tcp.src_port", be16(tr[0:2]))
		fmt.Println("tcp.dst_port", be16(tr[2:4]))
		fmt.Println("tcp.seq", be32(tr[4:8]))
		fmt.Println("tcp.ack", be32(tr[8:12]))
		fmt.Println("tcp.data_offset_bytes", off)
		fmt.Println("tcp.flags", tf)
		fmt.Println("tcp.window", be16(tr[14:16]))
		fmt.Println("payload.length", len(tr)-off)
	case proto == 17 && len(tr) >= 8:
		fmt.Println("udp.src_port", be16(tr[0:2]))
		fmt.Println("udp.dst_port", be16(tr[2:4]))
		fmt.Println("udp.length", be16(tr[4:6]))
		fmt.Println("payload.length", len(tr)-8)
	default:
		fmt.Println("payload.length", len(tr))
	}
}
