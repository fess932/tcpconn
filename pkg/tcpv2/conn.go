package tcpv2

import (
	"errors"
	"net"
	"sync"
	"tcpconn"
	"time"
)

const (
	DefaultWindowSize = 4096
	RetransmitTimeout = 200 * time.Millisecond
	MaxRetries        = 5
)

// Conn implements net.Conn over UDP with TCP-like reliability
type Conn struct {
	remoteAddr net.Addr
	localAddr  net.Addr
	conn       net.PacketConn // Underlying UDP connection

	// State Machine
	state *tcpconn.TCPStateMachine

	// Buffers
	readBuffer  *tcpconn.RingBuffer
	writeBuffer *tcpconn.RingBuffer

	// Sequence Numbers
	seqNum    uint32 // Next sequence number to send
	ackNum    uint32 // Next sequence number expected to receive
	remoteWin uint16 // Remote window size

	// Reliability
	sendQueue    map[uint32]*Packet // Unacknowledged packets
	receiveQueue map[uint32]*Packet // Out-of-order packets
	mu           sync.Mutex
	cond         *sync.Cond

	// Control
	closeChan chan struct{}
	closed    bool
}

// NewConn creates a new connection
func NewConn(conn net.PacketConn, remoteAddr net.Addr) *Conn {
	c := &Conn{
		conn:         conn,
		remoteAddr:   remoteAddr,
		localAddr:    conn.LocalAddr(),
		state:        tcpconn.NewTCPStateMachine(),
		sendQueue:    make(map[uint32]*Packet),
		receiveQueue: make(map[uint32]*Packet),
		closeChan:    make(chan struct{}),
		remoteWin:    DefaultWindowSize,
	}
	c.readBuffer, _ = tcpconn.NewRingBuffer(DefaultWindowSize)
	c.writeBuffer, _ = tcpconn.NewRingBuffer(DefaultWindowSize)
	c.cond = sync.NewCond(&c.mu)
	
	// Start retransmission loop
	go c.retransmitLoop()
	
	return c
}

// Read reads data from the connection
func (c *Conn) Read(b []byte) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for c.readBuffer.IsEmpty() {
		if c.closed || c.state.IsClosed() {
			return 0, net.ErrClosed
		}
		c.cond.Wait()
	}

	return c.readBuffer.Read(b)
}

// Write writes data to the connection
func (c *Conn) Write(b []byte) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed || c.state.IsClosed() {
		return 0, net.ErrClosed
	}

	// Simple implementation: send packet immediately
	// In a real TCP, we would write to writeBuffer and have a sender loop
	// Here we just packetize and send directly for simplicity, but tracking ACKs
	
	// Note: This is a simplified "Write" that acts more like "Send this data now"
	// rather than "Put in buffer and stream it". 
	// For a full stream implementation, we'd write to c.writeBuffer and have a background sender.
	
	// Let's implement a basic segmentation here
	totalSent := 0
	for totalSent < len(b) {
		chunkSize := 1000 // MSS-ish
		if len(b)-totalSent < chunkSize {
			chunkSize = len(b) - totalSent
		}
		
		chunk := b[totalSent : totalSent+chunkSize]
		packet := &Packet{
			Header: PacketHeader{
				SeqNum:     c.seqNum,
				AckNum:     c.ackNum,
				Flags:      FlagACK, // Always piggyback ACK
				WindowSize: uint16(c.readBuffer.FreeSpace()),
			},
			Data: chunk,
		}
		
		if err := c.sendPacketLocked(packet); err != nil {
			return totalSent, err
		}
		
		c.seqNum += uint32(len(chunk))
		totalSent += len(chunk)
	}

	return totalSent, nil
}

// Close closes the connection
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.state.ProcessEvent(tcpconn.CLOSE)
	c.sendControlPacket(FlagFIN | FlagACK)
	c.closed = true
	c.cond.Broadcast()
	close(c.closeChan)
	
	return nil
}

// LocalAddr returns the local network address
func (c *Conn) LocalAddr() net.Addr {
	return c.localAddr
}

// RemoteAddr returns the remote network address
func (c *Conn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// SetDeadline sets the read and write deadlines
func (c *Conn) SetDeadline(t time.Time) error {
	return errors.New("not implemented")
}

// SetReadDeadline sets the read deadline
func (c *Conn) SetReadDeadline(t time.Time) error {
	return errors.New("not implemented")
}

// SetWriteDeadline sets the write deadline
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return errors.New("not implemented")
}

// Internal methods

func (c *Conn) sendPacketLocked(p *Packet) error {
	data, err := p.Encode()
	if err != nil {
		return err
	}

	if _, err := c.conn.WriteTo(data, c.remoteAddr); err != nil {
		return err
	}

	// Store for retransmission if it has flags or data
	if len(p.Data) > 0 || p.Header.Flags&(FlagSYN|FlagFIN) != 0 {
		c.sendQueue[p.Header.SeqNum] = p
	}

	return nil
}

func (c *Conn) sendControlPacket(flags uint8) error {
	p := &Packet{
		Header: PacketHeader{
			SeqNum:     c.seqNum,
			AckNum:     c.ackNum,
			Flags:      flags,
			WindowSize: uint16(c.readBuffer.FreeSpace()),
		},
	}
	
	// SYN and FIN consume a sequence number
	if flags&(FlagSYN|FlagFIN) != 0 {
		c.seqNum++
	}
	
	return c.sendPacketLocked(p)
}

// HandlePacket processes an incoming packet
func (c *Conn) HandlePacket(p *Packet) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 1. Process Flags & State Machine
	if p.Header.Flags&FlagRST != 0 {
		c.state.ProcessEvent(tcpconn.RST)
		c.closed = true
		c.cond.Broadcast()
		return
	}

	if p.Header.Flags&FlagSYN != 0 {
		if c.state.GetState() == tcpconn.LISTEN {
			c.state.ProcessEvent(tcpconn.SYN)
			c.ackNum = p.Header.SeqNum + 1
			c.sendControlPacket(FlagSYN | FlagACK)
		} else if c.state.GetState() == tcpconn.SYN_SENT {
			c.state.ProcessEvent(tcpconn.SYN_ACK)
			c.ackNum = p.Header.SeqNum + 1
			c.sendControlPacket(FlagACK)
		}
	}

	if p.Header.Flags&FlagACK != 0 {
		if c.state.GetState() == tcpconn.SYN_RECEIVED {
			c.state.ProcessEvent(tcpconn.ACK)
		} else if c.state.GetState() == tcpconn.FIN_WAIT_1 {
			c.state.ProcessEvent(tcpconn.ACK)
		} else if c.state.GetState() == tcpconn.LAST_ACK {
			c.state.ProcessEvent(tcpconn.ACK)
			c.closed = true
			c.cond.Broadcast()
		}
		
		// Remove acknowledged packets from sendQueue
		// A simple cumulative ACK handling
		for seq, pkt := range c.sendQueue {
			pktEnd := seq
			if len(pkt.Data) > 0 {
				pktEnd += uint32(len(pkt.Data))
			} else if pkt.Header.Flags&(FlagSYN|FlagFIN) != 0 {
				pktEnd++
			}
			
			if p.Header.AckNum >= pktEnd {
				delete(c.sendQueue, seq)
			}
		}
	}

	if p.Header.Flags&FlagFIN != 0 {
		c.state.ProcessEvent(tcpconn.FIN)
		c.ackNum++
		c.sendControlPacket(FlagACK)
		c.cond.Broadcast() // Wake up readers to see EOF
	}

	// 2. Process Data
	if len(p.Data) > 0 {
		if p.Header.SeqNum == c.ackNum {
			// In-order packet
			c.readBuffer.Write(p.Data)
			c.ackNum += uint32(len(p.Data))
			c.cond.Broadcast() // Wake up readers

			// Check receive queue for next packets
			for {
				nextPkt, ok := c.receiveQueue[c.ackNum]
				if !ok {
					break
				}
				delete(c.receiveQueue, c.ackNum)
				c.readBuffer.Write(nextPkt.Data)
				c.ackNum += uint32(len(nextPkt.Data))
			}
			
			// Send ACK
			c.sendControlPacket(FlagACK)
		} else if p.Header.SeqNum > c.ackNum {
			// Out-of-order packet, buffer it
			c.receiveQueue[p.Header.SeqNum] = p
			// Send duplicate ACK to request retransmission (fast retransmit hint)
			c.sendControlPacket(FlagACK)
		}
	}
	
	// Update remote window
	c.remoteWin = p.Header.WindowSize
}

func (c *Conn) retransmitLoop() {
	ticker := time.NewTicker(RetransmitTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-c.closeChan:
			return
		case <-ticker.C:
			c.mu.Lock()
			for _, pkt := range c.sendQueue {
				// Retransmit
				// Note: In a real implementation we'd track retry counts and timeout
				data, _ := pkt.Encode()
				c.conn.WriteTo(data, c.remoteAddr)
			}
			c.mu.Unlock()
		}
	}
}
