package tcpv2

import (
	"errors"
	"fmt"
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
	conn       net.PacketConn

	state *tcpconn.TCPStateMachine

	readBuffer  *tcpconn.RingBuffer
	writeBuffer *tcpconn.RingBuffer

	seqNum    uint32
	ackNum    uint32
	remoteWin uint16

	sendQueue    map[uint32]*Packet
	receiveQueue map[uint32]*Packet
	mu           sync.Mutex
	cond         *sync.Cond

	closeChan chan struct{}
	closed    bool
}

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

	go c.retransmitLoop()

	return c
}

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

func (c *Conn) Write(b []byte) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed || c.state.IsClosed() {
		return 0, net.ErrClosed
	}

	totalSent := 0
	for totalSent < len(b) {
		chunkSize := 1000
		if len(b)-totalSent < chunkSize {
			chunkSize = len(b) - totalSent
		}

		chunk := b[totalSent : totalSent+chunkSize]
		packet := NewPacket(
			uint16(c.localAddr.(*net.UDPAddr).Port),
			uint16(c.remoteAddr.(*net.UDPAddr).Port),
			c.seqNum,
			c.ackNum,
			false, true, false, false, // SYN, ACK, FIN, RST
			uint16(c.readBuffer.FreeSpace()),
			chunk,
		)

		if err := c.sendPacketLocked(packet); err != nil {
			return totalSent, err
		}

		c.seqNum += uint32(len(chunk))
		totalSent += len(chunk)
	}

	return totalSent, nil
}

func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.state.ProcessEvent(tcpconn.CLOSE)
	c.sendControlPacket(false, true, true, false) // SYN, ACK, FIN, RST
	c.closed = true
	c.cond.Broadcast()
	close(c.closeChan)

	return nil
}

func (c *Conn) LocalAddr() net.Addr  { return c.localAddr }
func (c *Conn) RemoteAddr() net.Addr { return c.remoteAddr }

func (c *Conn) SetDeadline(t time.Time) error      { return errors.New("not implemented") }
func (c *Conn) SetReadDeadline(t time.Time) error  { return errors.New("not implemented") }
func (c *Conn) SetWriteDeadline(t time.Time) error { return errors.New("not implemented") }

func (c *Conn) sendPacketLocked(p *Packet) error {
	var srcIP, dstIP net.IP
	if addr, ok := c.localAddr.(*net.UDPAddr); ok {
		if ipv4 := addr.IP.To4(); ipv4 != nil {
			srcIP = ipv4
		} else {
			srcIP = addr.IP
		}
	}
	if addr, ok := c.remoteAddr.(*net.UDPAddr); ok {
		if ipv4 := addr.IP.To4(); ipv4 != nil {
			dstIP = ipv4
		} else {
			dstIP = addr.IP
		}
	}

	data, err := p.Encode(srcIP, dstIP)
	if err != nil {
		return fmt.Errorf("failed to encode packet in sendPacketLocked: %w", err)
	}

	if _, err := c.conn.WriteTo(data, c.remoteAddr); err != nil {
		return fmt.Errorf("failed to write packet to %s: %w", c.remoteAddr, err)
	}

	if len(p.Payload) > 0 || p.TCP.SYN || p.TCP.FIN {
		c.sendQueue[p.TCP.Seq] = p
	}

	return nil
}

func (c *Conn) sendControlPacket(syn, ack, fin, rst bool) error {
	p := NewPacket(
		uint16(c.localAddr.(*net.UDPAddr).Port),
		uint16(c.remoteAddr.(*net.UDPAddr).Port),
		c.seqNum,
		c.ackNum,
		syn, ack, fin, rst,
		uint16(c.readBuffer.FreeSpace()),
		nil,
	)

	if syn || fin {
		c.seqNum++
	}

	return c.sendPacketLocked(p)
}

func (c *Conn) HandlePacket(p *Packet) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if p.TCP.RST {
		c.state.ProcessEvent(tcpconn.RST)
		c.closed = true
		c.cond.Broadcast()
		return
	}

	if p.TCP.SYN {
		if c.state.GetState() == tcpconn.LISTEN {
			c.state.ProcessEvent(tcpconn.SYN)
			c.ackNum = p.TCP.Seq + 1
			c.sendControlPacket(true, true, false, false) // SYN-ACK
		} else if c.state.GetState() == tcpconn.SYN_SENT {
			c.state.ProcessEvent(tcpconn.SYN_ACK)
			c.ackNum = p.TCP.Seq + 1
			c.sendControlPacket(false, true, false, false) // ACK
		}
	}

	if p.TCP.ACK {
		if c.state.GetState() == tcpconn.SYN_RECEIVED {
			c.state.ProcessEvent(tcpconn.ACK)
		} else if c.state.GetState() == tcpconn.FIN_WAIT_1 {
			c.state.ProcessEvent(tcpconn.ACK)
		} else if c.state.GetState() == tcpconn.LAST_ACK {
			c.state.ProcessEvent(tcpconn.ACK)
			c.closed = true
			c.cond.Broadcast()
		}

		for seq, pkt := range c.sendQueue {
			pktEnd := seq
			if len(pkt.Payload) > 0 {
				pktEnd += uint32(len(pkt.Payload))
			} else if pkt.TCP.SYN || pkt.TCP.FIN {
				pktEnd++
			}

			if p.TCP.Ack >= pktEnd {
				delete(c.sendQueue, seq)
			}
		}
	}

	if p.TCP.FIN {
		c.state.ProcessEvent(tcpconn.FIN)
		c.ackNum++
		c.sendControlPacket(false, true, false, false) // ACK
		c.cond.Broadcast()
	}

	if len(p.Payload) > 0 {
		if p.TCP.Seq == c.ackNum {
			c.readBuffer.Write(p.Payload)
			c.ackNum += uint32(len(p.Payload))
			c.cond.Broadcast()

			for {
				nextPkt, ok := c.receiveQueue[c.ackNum]
				if !ok {
					break
				}
				delete(c.receiveQueue, c.ackNum)
				c.readBuffer.Write(nextPkt.Payload)
				c.ackNum += uint32(len(nextPkt.Payload))
			}

			c.sendControlPacket(false, true, false, false) // ACK
		} else if p.TCP.Seq > c.ackNum {
			c.receiveQueue[p.TCP.Seq] = p
			c.sendControlPacket(false, true, false, false) // ACK
		}
	}

	c.remoteWin = p.TCP.Window
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
				var srcIP, dstIP net.IP
				if addr, ok := c.localAddr.(*net.UDPAddr); ok {
					if ipv4 := addr.IP.To4(); ipv4 != nil {
						srcIP = ipv4
					} else {
						srcIP = addr.IP
					}
				}
				if addr, ok := c.remoteAddr.(*net.UDPAddr); ok {
					if ipv4 := addr.IP.To4(); ipv4 != nil {
						dstIP = ipv4
					} else {
						dstIP = addr.IP
					}
				}
				data, _ := pkt.Encode(srcIP, dstIP)
				c.conn.WriteTo(data, c.remoteAddr)
			}
			c.mu.Unlock()
		}
	}
}
