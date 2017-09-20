package goepoll

import (
	"testing"
)

func TestEpoll(t *testing.T) {
	srv, _ := NewServer(":8080")
	srv.Start(Hello)
}

func Hello(r *Received) int {
	r.body.WriteString("{Hello}")

	return 404
}
