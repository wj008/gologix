package enip

import (
	"errors"
	"time"
)

type TimeOut struct {
	ch      chan *Package
	timeOut time.Duration
}

func NewTimeOut(timeOut time.Duration) *TimeOut {
	return &TimeOut{
		ch:      make(chan *Package, 1),
		timeOut: timeOut,
	}
}

func (t *TimeOut) Read() (*Package, error) {
	for {
		select {
		case pack := <-t.ch:
			return pack, nil
		case <-time.After(t.timeOut):
			return nil, errors.New("超时读取数据")
		}
	}
}

func (t *TimeOut) Write(reply *Package) {
	t.ch <- reply
}

func (t *TimeOut) Close() {
	close(t.ch)
}
