package goepoll

import (
	"golang.org/x/sys/unix"
	"log"
	"os"
	"syscall"
)

const (
	EPOLLIN      = unix.EPOLLIN
	EPOLLOUT     = unix.EPOLLOUT
	EPOLLRDHUP   = unix.EPOLLRDHUP
	EPOLLPRI     = unix.EPOLLPRI
	EPOLLERR     = unix.EPOLLERR
	EPOLLHUP     = unix.EPOLLHUP
	EPOLLET      = unix.EPOLLET
	EPOLLONESHOT = unix.EPOLLONESHOT
)

const (
	maxWaitEventsBegin = 1024
)

type Hendler func(r *Received) int

type Bufer interface {
	Byte() []byte
	Write([]byte)
	GetAnswer() []byte
}

func init() {
	log.SetPrefix("GoEpoll: ")
	log.SetOutput(os.Stderr)
}

func fillSendBuffer(fd int, b []byte) (err error) {
	var x, n int
	for eagain := 0; eagain < 10; {
		x, err = unix.Write(fd, b[n:])
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
				err = nil
				eagain++
				continue
			}
			return
		} else if x <= 0 {
			return
		}
		//log.Println("Send", x)
		n += x
	}
	return
}

func fillReadBuffer(fd int, r *Received) (err error) {
	var x int
	for eagain := 0; eagain < 10; {
		x, err = unix.Read(fd, r.buf)
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
				err = nil
				eagain++
				continue
			}
			return
		} else if x <= 0 {
			return
		}
		r.r.Write(r.buf[:x])
		//log.Println("Read", x)
	}
	return
}
