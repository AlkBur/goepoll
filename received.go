package goepoll

import (
	"bytes"
	"log"
	"net/http"
	"strconv"
	"sync"
)

const READ_BUFFER_SIZE = 4096

var (
	strClose     = []byte("close")
	strKeepAlive = []byte("keep-alive")

	line200 = []byte("HTTP/1.1 200 OK\r\n")
	line400 = []byte("HTTP/1.1 400 Bad Request\r\n")
	line404 = []byte("HTTP/1.1 404 Not Found\r\n")

	endHead = []byte("\r\n\r\n")
)

const (
	textErrorContent = "Content-Type: text/plain; charset=utf-8\r\nX-Content-Type-Options: nosniff\r\n\r\n"
	//textBadRequest      = "HTTP/1.1 400 Bad Request\r\n" + textErrorContent
	//crlf                = "\r\n"
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

	Body []byte
	Url  []byte

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

	r.Body = nil
	r.Url = nil
}

func (r *Received) GetAnswer(code int) []byte {
	switch code {
	case http.StatusBadRequest:
		r.w.Write(line400)
		r.w.WriteString(textErrorContent)
	case http.StatusNotFound:
		r.w.Write(line404)
		r.w.WriteString(textErrorContent)
	default:
		r.w.Write(line200)
		r.w.WriteString("Server: goepoll/0.0.1\r\n")
		r.w.WriteString("Content-Length: ")
		r.w.WriteString(strconv.Itoa(r.body.Len()))
		r.w.WriteString("\r\nConnection: ")
		if r.isKeepAlive {
			r.w.Write(strKeepAlive)
		} else {
			r.w.Write(strClose)
		}
		r.w.WriteString("\r\nContent-Type: application/json\r\n\r\n")
		r.w.Write(r.body.Bytes())

	}

	return r.w.Bytes()
}

func (r *Received) SetSettings() bool {
	b := r.r.Bytes()
	if b[0] == 'G' {
		r.isGet = true
	} else {
		r.isGet = false
	}
	i := bytes.Index(b, endHead)
	if i > 0 {
		i += 4
		r.Body = b[i:]
	} else {
		log.Println("Error ended http header")
		return false
	}
	r.isKeepAlive = bytes.Contains(b, strKeepAlive)

	i = bytes.IndexByte(b[:i], '\r')
	if i > 0 {
		r.Url = b[4:i]
		//log.Println("get host:", string(b))
		i = bytes.IndexByte(r.Url, ' ')
		if i > 0 {
			r.Url = r.Url[:i]
		}
	} else {
		return false
	}
	return true
}

func (r *Received) IsGet() bool {
	return r.isGet
}

func (r *Received) WriteString(str string) {
	r.body.WriteString(str)
}

func (r *Received) Write(p []byte) {
	r.body.Write(p)
}

func (r *Received) WriteByte(b byte) {
	r.body.WriteByte(b)
}
