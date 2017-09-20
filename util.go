package goepoll

import (
	"golang.org/x/sys/unix"
	"net"
	"syscall"
)

func temporaryErr(err error) bool {
	errno, ok := err.(syscall.Errno)
	if !ok {
		return false
	}
	return errno.Temporary()
}

func resolveSockAddr4(netaddr string) (unix.Sockaddr, error) {
	addr, err := net.ResolveTCPAddr("tcp4", netaddr)
	if err != nil {
		return nil, err
	}
	ip := addr.IP
	if len(ip) == 0 {
		ip = net.IPv4zero
	}
	sa4 := &unix.SockaddrInet4{Port: addr.Port}
	copy(sa4.Addr[:], ip.To4())
	return sa4, nil
}
