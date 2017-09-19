package goepoll

import (
	"fmt"
	"golang.org/x/sys/unix"
	"log"
	"sync"
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

	_EPOLLCLOSED = 0x20
)

const (
	maxWaitEventsBegin = 1024
)

var (
	ErrClosed     = fmt.Errorf("Epoll instance is closed")
	ErrRegistered = fmt.Errorf("File descriptor is already registered in Epoll instance")
)

type Bufer interface {
	Byte() []byte
	Write([]byte)
	GetAnswer() []byte
}

type EpollEvent uint32

type Epoll struct {
	mu sync.RWMutex

	fd       int
	eventFd  int
	closed   bool
	waitDone chan struct{}

	callbacks map[int]func(EpollEvent)

	timeout int
}

func New(timeout int) (*Epoll, error) {
	fd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}

	r0, _, errno := unix.Syscall(unix.SYS_EVENTFD2, 0, 0, 0)
	if errno != 0 {
		return nil, errno
	}
	eventFd := int(r0)

	err = unix.EpollCtl(fd, unix.EPOLL_CTL_ADD, eventFd, &unix.EpollEvent{
		Events: unix.EPOLLIN,
		Fd:     int32(eventFd),
	})
	if err != nil {
		unix.Close(fd)
		unix.Close(eventFd)
		return nil, err
	}

	ep := &Epoll{
		fd:        fd,
		eventFd:   eventFd,
		callbacks: make(map[int]func(EpollEvent)),
		waitDone:  make(chan struct{}),
		timeout:   timeout,
	}

	return ep, nil
}

func (ep *Epoll) wait() {
	defer func() {
		if err := unix.Close(ep.fd); err != nil {
			log.Println("Error close connect:", err)
		}
		close(ep.waitDone)
	}()

	events := make([]unix.EpollEvent, maxWaitEventsBegin)
	callbacks := make([]func(EpollEvent), 0, maxWaitEventsBegin)

	for {
		n, err := unix.EpollWait(ep.fd, events, ep.timeout)
		if err != nil {
			if temporaryErr(err) {
				continue
			}
			log.Println("Error wait:", err)
			return
		}

		callbacks = callbacks[:n]

		ep.mu.RLock()
		for i := 0; i < n; i++ {
			fd := int(events[i].Fd)
			if fd == ep.eventFd { // signal to close
				ep.mu.RUnlock()
				return
			}
			callbacks[i] = ep.callbacks[fd]
		}
		ep.mu.RUnlock()

		for i := 0; i < n; i++ {
			if cb := callbacks[i]; cb != nil {
				cb(EpollEvent(events[i].Events))
				callbacks[i] = nil
			}
		}

		if n == len(events) {
			events = make([]unix.EpollEvent, n*2)
			callbacks = make([]func(EpollEvent), 0, n*2)
		}
	}
}

func listen(port int) (ln int, err error) {
	ln, err = unix.Socket(unix.AF_INET, unix.O_NONBLOCK|unix.SOCK_STREAM, 0)
	if err != nil {
		return
	}

	// Need for avoid receiving EADDRINUSE error.
	// Closed listener could be in TIME_WAIT state some time.
	unix.SetsockoptInt(ln, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)

	addr := &unix.SockaddrInet4{
		Port: port,
		Addr: [4]byte{0, 0, 0, 0}, // 0.0.0.0 - ANY
	}

	if err = unix.Bind(ln, addr); err != nil {
		return
	}
	err = unix.Listen(ln, 4)

	return
}

func (ep *Epoll) Add(fd int, events EpollEvent, cb func(EpollEvent)) (err error) {
	ev := &unix.EpollEvent{
		Events: uint32(events),
		Fd:     int32(fd),
	}

	ep.mu.Lock()
	defer ep.mu.Unlock()

	if ep.closed {
		return ErrClosed
	}
	if _, has := ep.callbacks[fd]; has {
		return ErrRegistered
	}
	ep.callbacks[fd] = cb

	return unix.EpollCtl(ep.fd, unix.EPOLL_CTL_ADD, fd, ev)
}

func (ep *Epoll) Start(port int) error {
	ln, err := listen(port)
	if err != nil {
		return err
	}
	defer unix.Close(ln)

	ep.Add(ln, EPOLLIN, func(evt EpollEvent) {
		if evt&_EPOLLCLOSED != 0 {
			return
		}

		// Accept new incoming connection.
		conn, _, err := unix.Accept(ln)
		if err != nil {
			log.Printf("could not accept: %s\n", err.Error())
		}

		// Socket must not block read() from it.
		unix.SetNonblock(conn, true)

		// Add connection fd to epoll instance to get notifications about
		// available data.
		ep.Add(conn, EPOLLIN|EPOLLET|EPOLLHUP|EPOLLRDHUP, func(evt EpollEvent) {
			// If EPOLLRDHUP is supported, it will be triggered after conn
			// close() or shutdown(). In older versions EPOLLHUP is triggered.
			if evt&_EPOLLCLOSED != 0 {
				return
			}

			r := getReceived()
			buf := r.buf

			for {
				n, _ := unix.Read(conn, buf)
				if n == 0 {
					unix.Close(conn)
				}
				if n <= 0 {
					break
				}
				r.r.Write(buf[:n])
			}
			buf = r.GetAnswer()
			unix.Write(conn, buf)

			putReceived(r)

			unix.Close(conn)
		})
	})

	// Run wait loop.
	ep.wait()

	return nil
}
