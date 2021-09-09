package future

import (
	"errors"
	"testing"
	"time"
)

func TestResult(t *testing.T) {
	r := NewResult()
	select {
	case <-r.Done():
		t.Fatalf("result is done before notify")
	default:
	}
	var errAborted = errors.New("aborted")
	start := time.Now()
	go func() {
		time.Sleep(time.Millisecond * 50)
		r.Notify(errAborted)
	}()
	err := r.Err()
	if time.Since(start) < time.Millisecond*50 {
		t.Fatalf("received err before notify")
	}
	if err != errAborted {
		t.Fatalf("received err %v does not match the expected err %v", err, errAborted)
	}

}
