package goepoll

import (
	"bytes"
	"strconv"
	"sync"
	"syscall"
)

const READ_BUFFER_SIZE = 4096

var (
	strClose     = []byte(`close`)
	strKeepAlive = []byte(`keep-alive`)

	line200 = []byte("HTTP/1.1 200 OK\r\n")
	line400 = []byte("HTTP/1.1 400 Bad Request\r\n")
	line404 = []byte("HTTP/1.1 404 Not Found\r\n")

	endHead = []byte("\r\n\r\n")
)

var receivedPool = sync.Pool{
	New: func() interface{} {
		return &Received{
			r:    new(bytes.Buffer),
			w:    new(bytes.Buffer),
			body: new(bytes.Buffer),
			buf:  make([]byte, READ_BUFFER_SIZE),
		}
	},
}

type Received struct {
	buf  []byte
	r    *bytes.Buffer
	w    *bytes.Buffer
	body *bytes.Buffer

	isKeepAlive bool
	isGet       bool
}

func getReceived() *Received {
	return receivedPool.Get().(*Received)
}

func putReceived(r *Received) {
	// сброс
	r.r.Reset()
	r.w.Reset()
	r.body.Reset()
	r.buf = r.buf[:0]
}

func (r *Received) GetAnswer() []byte {
	r.w.Write(line200)
	r.w.WriteString("Server: goepoll/0.0.1\r\n")
	r.w.WriteString("Content-Length: ")
	r.w.WriteString(strconv.Itoa(r.body.Len()))
	r.w.WriteString("\r\nConnection: ")
	if r.isKeepAlive {
		r.w.WriteString("keep-alive")
	} else {
		r.w.WriteString("close")
	}
	r.w.WriteString("\r\nContent-Type: application/json\r\n\r\n")
	r.w.Write(r.body.Bytes())

	return r.w.Bytes()
}

func temporaryErr(err error) bool {
	errno, ok := err.(syscall.Errno)
	if !ok {
		return false
	}
	return errno.Temporary()
}
