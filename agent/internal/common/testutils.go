package common

import (
	"sync"
	"testing"
	"time"
)

func WaitWithTimeout(t *testing.T, wg *sync.WaitGroup, timeout time.Duration) {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return
	case <-time.After(timeout):
		t.Errorf("Test timeout after (%s)", timeout)
		return
	}
}
