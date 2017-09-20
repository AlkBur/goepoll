package goepoll

import (
	"golang.org/x/sys/unix"
	"log"
)

type Server struct {
	timeout  int
	listenfd int
}

func NewServer(addr string) (*Server, error) {
	server := &Server{
		timeout: -1,
	}
	return server, server.listen(addr)
}

func (s *Server) SetTimeout(time int) {
	s.timeout = time
}

func (s *Server) listen(addr string) error {
	lfd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM|unix.SOCK_NONBLOCK, unix.IPPROTO_TCP)
	if err != nil {
		return err
	}
	sa4, err := resolveSockAddr4(addr)
	if err != nil {
		return err
	}
	if err = unix.Bind(lfd, sa4); err != nil {
		unix.Close(lfd)
		return err
	}
	if err = unix.Listen(lfd, unix.SOMAXCONN); err != nil {
		unix.Close(lfd)
		return err
	}
	if err = unix.SetNonblock(lfd, true); err != nil {
		unix.Close(lfd)
		return err
	}
	if err = unix.SetsockoptInt(lfd, unix.IPPROTO_TCP, unix.TCP_NODELAY, 1); err != nil {
		unix.Close(lfd)
		return err
	}
	if err = unix.SetsockoptInt(lfd, unix.IPPROTO_TCP, unix.TCP_QUICKACK, 1); err != nil {
		unix.Close(lfd)
		return err
	}

	s.listenfd = lfd
	return nil
}

func (s *Server) Start() error {
	var event unix.EpollEvent

	efd, err := unix.EpollCreate1(0)
	if err != nil {
		return err
	}

	event.Fd = int32(s.listenfd)
	event.Events = EPOLLIN | EPOLLET

	err = unix.EpollCtl(efd, unix.EPOLL_CTL_ADD, s.listenfd, &event)
	if err != nil {
		return err
	}

	events := make([]unix.EpollEvent, maxWaitEventsBegin)

	/* The event loop */
	var i, n int
	for {
		n, err = unix.EpollWait(efd, events, s.timeout)
		if err != nil {
			if temporaryErr(err) {
				continue
			}
			log.Println("Error wait:", err)
			break
		}
		for i = 0; i < n; i++ {
			if (events[i].Events&EPOLLERR) != 0 ||
				(events[i].Events&EPOLLHUP) != 0 ||
				!((events[i].Events & EPOLLIN) != 0) {
				/* An error has occured on this fd, or the socket is not
				   ready for reading (why were we notified then?) */
				log.Println("epoll error:", err)
				unix.Close(int(events[i].Fd))
				continue
			} else if s.listenfd == int(events[i].Fd) {
				for {
					var infd int
					infd, _, err = unix.Accept(s.listenfd)
					if err != nil {
						if (err == unix.EAGAIN) || (err == unix.EWOULDBLOCK) {
							/* We have processed all incoming
							   connections. */
							break
						} else {
							log.Println("Error wait accept:", err)
							break
						}
					}
					// Socket must not block read() from it.
					unix.SetNonblock(infd, true)

					event.Fd = int32(infd)
					event.Events = EPOLLIN | EPOLLET
					err = unix.EpollCtl(efd, unix.EPOLL_CTL_ADD, infd, &event)
					if err != nil {
						log.Println("Error wait EpollCtl:", err)
					}
				}
			} else {
				/* We have data on the fd waiting to be read. Read and
				display it. We must read whatever data is available
				completely, as we are running in edge-triggered mode
				and won't get a notification again for the same
				data. */
				r := getReceived()

				for {
					var count int
					count, err = unix.Read(int(events[i].Fd), r.buf)
					if err != nil {
						if err != unix.EAGAIN {
							log.Println("Error read:", err)
						}
						break
					} else if count == 0 {
						/* End of file. The remote has closed the
						   connection. */
						break
					}
					r.r.Write(r.buf[:count])
				}

				if r.r.Len() > 0 {
					go ServerHandle(int(events[i].Fd), r)
				} else {
					putReceived(r)
					unix.Close(int(events[i].Fd))
				}
			}
		}
	}

	return nil
}

func ServerHandle(fd int, r *Received) {
	closeConn := true
	defer func() {
		putReceived(r)
		if closeConn {
			unix.Close(fd)
		}
	}()
	r.SetSettings()

	log.Println(string(r.Url))

	b := r.GetAnswer()
	err := fillSendBuffer(fd, b)
	if err != nil {
		closeConn = true
	}
}
