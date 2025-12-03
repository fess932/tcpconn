package tcpv2

import (
	"fmt"
	"net"
	"sync"
	"tcpconn"
	"time"
)

// Listener implements a TCP-like listener over UDP
type Listener struct {
	conn    net.PacketConn
	conns   map[string]*Conn
	mu      sync.Mutex
	accept  chan *Conn
	closed  bool
}

// Listen creates a new Listener
func Listen(address string) (*Listener, error) {
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		return nil, err
	}

	l := &Listener{
		conn:   conn,
		conns:  make(map[string]*Conn),
		accept: make(chan *Conn, 10),
	}

	go l.readLoop()

	return l, nil
}

// Accept waits for and returns the next connection to the listener
func (l *Listener) Accept() (*Conn, error) {
	c, ok := <-l.accept
	if !ok {
		return nil, net.ErrClosed
	}
	return c, nil
}

// Close closes the listener
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

// Addr returns the listener's network address
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

		// Decode packet to check flags
		packet, err := DecodePacket(buf[:n])
		if err != nil {
			continue
		}

		l.mu.Lock()
		c, exists := l.conns[addr.String()]
		if !exists {
			// Handle new connection (SYN)
			if packet.Header.Flags&FlagSYN != 0 {
				c = NewConn(l.conn, addr)
				l.conns[addr.String()] = c
				
				// Transition to LISTEN state to accept SYN
				c.state.ProcessEvent(tcpconn.PASSIVE_OPEN)
				
				// Pass to Accept loop
				l.accept <- c
			}
		}
		l.mu.Unlock()

		if c != nil {
			c.HandlePacket(packet)
		}
	}
}

// Dial connects to a remote address
func Dial(address string) (*Conn, error) {
	// Resolve address
	raddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	// Create a UDP connection
	conn, err := net.ListenPacket("udp", ":0") // Ephemeral port
	if err != nil {
		return nil, err
	}

	c := NewConn(conn, raddr)
	
	// Start read loop for client
	go func() {
		buf := make([]byte, 65535)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			
			// Verify sender
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

	// Start Active Open
	if err := c.state.ProcessEvent(tcpconn.ACTIVE_OPEN); err != nil {
		return nil, err
	}
	
	// Send SYN
	c.seqNum = 100 
	if err := c.sendControlPacket(FlagSYN); err != nil {
		return nil, err
	}

	// Wait for ESTABLISHED state
	for i := 0; i < 50; i++ {
		if c.state.IsConnected() {
			return c, nil
		}
		if c.state.IsClosed() && i > 5 {
			return nil, fmt.Errorf("connection refused or reset")
		}
		time.Sleep(100 * time.Millisecond)
	}

	return nil, fmt.Errorf("handshake timeout")
}
