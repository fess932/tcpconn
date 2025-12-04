package tcpv2

import (
	"fmt"
	"net"
	"sync"
	"tcpconn"
	"time"
)

type Listener struct {
	conn   net.PacketConn
	conns  map[string]*Conn
	mu     sync.Mutex
	accept chan *Conn
	closed bool
}

func Listen(address string) (*Listener, error) {
	conn, err := net.ListenPacket("udp4", address)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	l := &Listener{
		conn:   conn,
		conns:  make(map[string]*Conn),
		accept: make(chan *Conn, 10),
	}

	go l.readLoop()

	return l, nil
}

func (l *Listener) Accept() (*Conn, error) {
	c, ok := <-l.accept
	if !ok {
		return nil, net.ErrClosed
	}
	return c, nil
}

func (l *Listener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil
	}

	l.closed = true
	close(l.accept)
	return l.conn.Close()
}

func (l *Listener) Addr() net.Addr {
	return l.conn.LocalAddr()
}

func (l *Listener) readLoop() {
	buf := make([]byte, 65535)
	for {
		n, addr, err := l.conn.ReadFrom(buf)
		if err != nil {
			if !l.closed {
				fmt.Printf("Listener read error: %v\n", err)
			}
			return
		}

		packet, err := DecodePacket(buf[:n])
		if err != nil {
			continue
		}

		l.mu.Lock()
		c, exists := l.conns[addr.String()]
		if !exists {
			if packet.TCP.SYN {
				c = NewConn(l.conn, addr)
				l.conns[addr.String()] = c
				c.state.ProcessEvent(tcpconn.PASSIVE_OPEN)
				l.accept <- c
			}
		}
		l.mu.Unlock()

		if c != nil {
			c.HandlePacket(packet)
		}
	}
}

func Dial(address string) (net.Conn, error) {
	raddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve UDP address %s: %w", address, err)
	}

	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP client socket: %w", err)
	}

	c := NewConn(conn, raddr)

	go func() {
		buf := make([]byte, 65535)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}

			if addr.String() != raddr.String() {
				continue
			}

			packet, err := DecodePacket(buf[:n])
			if err != nil {
				continue
			}

			c.HandlePacket(packet)
		}
	}()

	if err := c.state.ProcessEvent(tcpconn.ACTIVE_OPEN); err != nil {
		return nil, fmt.Errorf("failed to process ACTIVE_OPEN event: %w", err)
	}

	c.seqNum = 100
	if err := c.sendControlPacket(true, false, false, false); err != nil { // SYN
		return nil, fmt.Errorf("failed to send SYN packet: %w", err)
	}

	select {
	case <-c.connected:
		return c, nil

	case <-c.reset:
		return nil, fmt.Errorf("connection reset by peer")

	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("handshake timeout")
	}
}
