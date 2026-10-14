package lab

import (
	"net"
	"sync"
)

// Listen bounds accepted connections as well as the handler admission limit.
func Listen() (net.Listener, error) {
	l, err := net.Listen("tcp", "127.0.0.1:8081")
	if err != nil {
		return nil, err
	}
	return &limitedListener{Listener: l, slots: make(chan struct{}, 128), done: make(chan struct{})}, nil
}

type limitedListener struct {
	net.Listener
	slots chan struct{}
	done  chan struct{}
	once  sync.Once
}

func (l *limitedListener) Accept() (net.Conn, error) {
	select {
	case l.slots <- struct{}{}:
	case <-l.done:
		return nil, net.ErrClosed
	}
	conn, err := l.Listener.Accept()
	if err != nil {
		<-l.slots
		return nil, err
	}
	return &limitedConn{Conn: conn, release: func() { <-l.slots }}, nil
}
func (l *limitedListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return l.Listener.Close()
}

type limitedConn struct {
	net.Conn
	once    sync.Once
	release func()
}

func (c *limitedConn) Close() error { err := c.Conn.Close(); c.once.Do(c.release); return err }
