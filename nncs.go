package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strings"
)

var nncsPorts = []int{10025, 10125}

func startNNCS() {
	if os.Getenv("NNCS_ENABLE") == "0" {
		fmt.Println("[NNCS] disabled (NNCS_ENABLE=0)")
		return
	}
	for _, ip := range strings.Split(envOr("NNCS_BIND", "0.0.0.0"), ",") {
		ip = strings.TrimSpace(ip)
		bind := net.ParseIP(ip)
		if bind == nil {
			fmt.Printf("[NNCS] ignoring invalid NNCS_BIND address %q\n", ip)
			continue
		}
		reported := bind
		if bind.IsUnspecified() {
			reported = net.ParseIP(envOr("NNCS_SERVER_IP", nextendoHost))
		}
		for _, port := range nncsPorts {
			conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: bind, Port: port})
			if err != nil {
				fmt.Printf("[NNCS] cannot listen on %s:%d: %v\n", ip, port, err)
				continue
			}
			fmt.Printf("[NNCS] NAT-check responder on UDP %s:%d\n", ip, port)
			go serveNNCS(conn, ipv4ToU32(reported))
		}
	}
}

func serveNNCS(conn *net.UDPConn, serverIP uint32) {
	buf := make([]byte, 128)
	for {
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		if n < 16 {
			continue
		}
		reply := make([]byte, 16)
		copy(reply[0:4], buf[0:4])
		binary.BigEndian.PutUint32(reply[4:8], uint32(src.Port))
		binary.BigEndian.PutUint32(reply[8:12], ipv4ToU32(src.IP))
		binary.BigEndian.PutUint32(reply[12:16], serverIP)
		if _, err := conn.WriteToUDP(reply, src); err != nil {
			fmt.Printf("[NNCS] reply to %s failed: %v\n", src, err)
		}
	}
}

func ipv4ToU32(ip net.IP) uint32 {
	if v4 := ip.To4(); v4 != nil {
		return binary.BigEndian.Uint32(v4)
	}
	return 0
}
