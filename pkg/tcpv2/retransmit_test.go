package tcpv2

import (
	"net"
	"sync"
	"sync/atomic"
	"tcpconn"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// LossyPacketConn wraps net.PacketConn and drops packets randomly
type LossyPacketConn struct {
	net.PacketConn
	dropCount   atomic.Int32
	totalCount  atomic.Int32
	dropEveryN  int32 // Drop every Nth packet
	mu          sync.Mutex
}

func NewLossyPacketConn(conn net.PacketConn, dropEveryN int32) *LossyPacketConn {
	return &LossyPacketConn{
		PacketConn: conn,
		dropEveryN: dropEveryN,
	}
}

func (l *LossyPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	count := l.totalCount.Add(1)
	
	// Drop every Nth packet
	if l.dropEveryN > 0 && count%l.dropEveryN == 0 {
		l.dropCount.Add(1)
		// Pretend we sent it
		return len(p), nil
	}
	
	return l.PacketConn.WriteTo(p, addr)
}

func (l *LossyPacketConn) GetStats() (dropped, total int32) {
	return l.dropCount.Load(), l.totalCount.Load()
}

func TestRetransmission_WithPacketLoss(t *testing.T) {
	// Start server
	serverListener, err := net.ListenPacket("udp4", "127.0.0.1:0")
	require.NoError(t, err)
	defer serverListener.Close()

	serverAddr := serverListener.LocalAddr().String()

	// Server goroutine
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)

		// Accept connection
		buf := make([]byte, 65535)
		n, clientAddr, err := serverListener.ReadFrom(buf)
		if err != nil {
			return
		}

		// Decode SYN
		synPkt, err := DecodePacket(buf[:n])
		if err != nil || !synPkt.TCP.SYN {
			return
		}

		// Create server connection
		serverConn := NewConn(serverListener, clientAddr)
		serverConn.state.ProcessEvent(tcpconn.PASSIVE_OPEN)
		serverConn.ackNum = synPkt.TCP.Seq + 1

		// Send SYN-ACK
		serverConn.sendControlPacket(true, true, false, false)

		// Read incoming packets in background
		go func() {
			buf := make([]byte, 65535)
			for {
				n, addr, err := serverListener.ReadFrom(buf)
				if err != nil {
					return
				}
				if addr.String() != clientAddr.String() {
					continue
				}
				pkt, err := DecodePacket(buf[:n])
				if err != nil {
					continue
				}
				serverConn.HandlePacket(pkt)
			}
		}()

		// Wait for data
		time.Sleep(100 * time.Millisecond)
		dataBuf := make([]byte, 1024)
		n, err = serverConn.Read(dataBuf)
		if err == nil {
			// Echo back
			serverConn.Write(dataBuf[:n])
		}
	}()

	// Client with lossy connection (drop every 3rd packet)
	clientConn, err := net.ListenPacket("udp4", ":0")
	require.NoError(t, err)
	defer clientConn.Close()

	lossyConn := NewLossyPacketConn(clientConn, 3)

	raddr, err := net.ResolveUDPAddr("udp4", serverAddr)
	require.NoError(t, err)

	client := NewConn(lossyConn, raddr)

	// Read loop
	go func() {
		buf := make([]byte, 65535)
		for {
			n, addr, err := clientConn.ReadFrom(buf)
			if err != nil {
				return
			}
			if addr.String() != raddr.String() {
				continue
			}
			pkt, err := DecodePacket(buf[:n])
			if err != nil {
				continue
			}
			client.HandlePacket(pkt)
		}
	}()

	// Initiate connection
	err = client.state.ProcessEvent(tcpconn.ACTIVE_OPEN)
	require.NoError(t, err)

	client.seqNum = 100
	err = client.sendControlPacket(true, false, false, false)
	require.NoError(t, err)

	// Wait for connection
	time.Sleep(200 * time.Millisecond)
	require.True(t, client.state.IsConnected(), "Connection should be established despite packet loss")

	// Send data
	testData := []byte("Hello with packet loss!")
	n, err := client.Write(testData)
	require.NoError(t, err)
	require.Equal(t, len(testData), n)

	// Read echo
	buf := make([]byte, 1024)
	client.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err = client.Read(buf)
	require.NoError(t, err)
	require.Equal(t, testData, buf[:n])

	// Check that some packets were dropped and retransmitted
	dropped, total := lossyConn.GetStats()
	t.Logf("Packet loss stats: %d dropped out of %d total (%.1f%%)", dropped, total, float64(dropped)/float64(total)*100)
	require.Greater(t, dropped, int32(0), "Some packets should have been dropped")

	client.Close()
	<-serverDone
}

func TestRTO_Adaptation(t *testing.T) {
	mockConn := NewMockPacketConn()
	remoteAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1").To4(), Port: 12345}
	c := NewConn(mockConn, remoteAddr)
	defer c.Close()

	// Set to ESTABLISHED
	c.state.ProcessEvent(tcpconn.PASSIVE_OPEN)
	c.state.ProcessEvent(tcpconn.SYN)
	c.state.ProcessEvent(tcpconn.ACK)

	c.seqNum = 100
	c.ackNum = 200

	// Initial RTO should be InitialRTO
	require.Equal(t, InitialRTO, c.rto)

	// Simulate first RTT measurement (100ms)
	c.updateRTO(100 * time.Millisecond)
	
	// After first measurement: SRTT = RTT, RTTVAR = RTT/2
	// RTO = SRTT + 4*RTTVAR = 100ms + 4*50ms = 300ms
	require.Equal(t, 100*time.Millisecond, c.srtt)
	require.Equal(t, 50*time.Millisecond, c.rttvar)
	expectedRTO := 100*time.Millisecond + 4*50*time.Millisecond
	require.Equal(t, expectedRTO, c.rto)

	// Simulate second RTT measurement (150ms)
	c.updateRTO(150 * time.Millisecond)
	
	// SRTT should increase, RTTVAR should reflect variance
	require.Greater(t, c.srtt, 100*time.Millisecond)
	require.Less(t, c.srtt, 150*time.Millisecond)
	
	// RTO should be updated
	require.Greater(t, c.rto, expectedRTO)
	
	t.Logf("After 2 measurements: SRTT=%v, RTTVAR=%v, RTO=%v", c.srtt, c.rttvar, c.rto)
}

func TestRTO_Bounds(t *testing.T) {
	mockConn := NewMockPacketConn()
	remoteAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1").To4(), Port: 12345}
	c := NewConn(mockConn, remoteAddr)
	defer c.Close()

	// Test minimum bound
	c.updateRTO(1 * time.Millisecond)
	require.GreaterOrEqual(t, c.rto, MinRTO, "RTO should not go below MinRTO")

	// Test maximum bound
	c.updateRTO(100 * time.Second)
	require.LessOrEqual(t, c.rto, MaxRTO, "RTO should not exceed MaxRTO")
}
