package goepoll

import (
	"testing"
)

func TestEpoll(t *testing.T) {
	srv, _ := NewServer(":8080")
	srv.Start()
}
