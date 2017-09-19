package goepoll

import (
	"testing"
)

func TestEpoll(t *testing.T) {
	srv, err := New(-1)
	if err != nil {
		t.Fatal(err)
	}
	err = srv.Start(8080)
	if err != nil {
		t.Fatal(err)
	}
	//done := make(chan struct{})
	//<-done
}
