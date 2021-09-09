package future

type Result struct {
	ch  chan struct{}
	err error
}

func NewResult() *Result {
	return &Result{ch: make(chan struct{})}
}

func (r *Result) Notify(err error) {
	select {
	case <-r.ch:
	default:
		r.err = err
		close(r.ch)
	}
}

func (r *Result) Done() <-chan struct{} {
	return r.ch
}

func (r *Result) Err() error {
	<-r.ch
	return r.err
}
