package tcpv2

import (
	"net"
	"sync"
	"tcpconn"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// MockPacketConn implements net.PacketConn for testing
type MockPacketConn struct {
	mu        sync.Mutex
	packets   [][]byte
	addrs     []net.Addr
	readCh    chan struct{}
	closed    bool
	localAddr net.Addr
}

func NewMockPacketConn() *MockPacketConn {
	return &MockPacketConn{
		readCh:    make(chan struct{}, 100),
		localAddr: &net.UDPAddr{IP: net.ParseIP("127.0.0.1").To4(), Port: 8080},
	}
}

func (m *MockPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	<-m.readCh
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return 0, nil, net.ErrClosed
	}

	if len(m.packets) == 0 {
		return 0, nil, nil
	}

	pkt := m.packets[0]
	src := m.addrs[0]
	m.packets = m.packets[1:]
	m.addrs = m.addrs[1:]

	copy(p, pkt)
	return len(pkt), src, nil
}

func (m *MockPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(p), nil
}

func (m *MockPacketConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	close(m.readCh)
	return nil
}

func (m *MockPacketConn) LocalAddr() net.Addr {
	return m.localAddr
}

func (m *MockPacketConn) SetDeadline(t time.Time) error      { return nil }
func (m *MockPacketConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *MockPacketConn) SetWriteDeadline(t time.Time) error { return nil }

func (m *MockPacketConn) InjectPacket(data []byte, addr net.Addr) {
	m.mu.Lock()
	m.packets = append(m.packets, data)
	m.addrs = append(m.addrs, addr)
	m.mu.Unlock()
	m.readCh <- struct{}{}
}

func TestNewConn(t *testing.T) {
	mockConn := NewMockPacketConn()
	remoteAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1").To4(), Port: 12345}

	c := NewConn(mockConn, remoteAddr)
	require.NotNil(t, c)
	require.NotNil(t, c.state)
	require.NotNil(t, c.readBuffer)
	require.NotNil(t, c.writeBuffer)
	require.Equal(t, remoteAddr, c.remoteAddr)

	c.Close()
}

func TestConn_Handshake(t *testing.T) {
	mockConn := NewMockPacketConn()
	remoteAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1").To4(), Port: 12345}

	c := NewConn(mockConn, remoteAddr)
	defer c.Close()

	// Simulate server side: LISTEN
	err := c.state.ProcessEvent(tcpconn.PASSIVE_OPEN)
	require.NoError(t, err)
	require.Equal(t, tcpconn.LISTEN, c.state.GetState())

	// Receive SYN
	synPkt := NewPacket(12345, 8080, 100, 0, true, false, false, false, 4096, nil)

	c.HandlePacket(synPkt)

	// Should transition to SYN_RECEIVED
	require.Equal(t, tcpconn.SYN_RECEIVED, c.state.GetState())
	require.Equal(t, uint32(101), c.ackNum) // SYN consumes 1 seq
}

func TestConn_DataTransfer(t *testing.T) {
	mockConn := NewMockPacketConn()
	remoteAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1").To4(), Port: 12345}
	c := NewConn(mockConn, remoteAddr)
	defer c.Close()

	// Manually set to ESTABLISHED
	c.state.ProcessEvent(tcpconn.PASSIVE_OPEN) // LISTEN
	c.state.ProcessEvent(tcpconn.SYN)          // SYN_RECEIVED
	c.state.ProcessEvent(tcpconn.ACK)          // ESTABLISHED

	c.seqNum = 200
	c.ackNum = 101

	// Receive Data
	dataPkt := NewPacket(12345, 8080, 101, 200, false, true, false, false, 4096, []byte("Hello"))

	// Inject packet in background
	go func() {
		time.Sleep(10 * time.Millisecond)
		c.HandlePacket(dataPkt)
	}()

	// Read
	buf := make([]byte, 1024)
	n, err := c.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.Equal(t, "Hello", string(buf[:n]))

	require.Equal(t, uint32(106), c.ackNum)
}

func TestConn_Write(t *testing.T) {
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

	// Write data
	data := []byte("Test Data")
	n, err := c.Write(data)
	require.NoError(t, err)
	require.Equal(t, len(data), n)

	// Verify seqNum incremented
	require.Equal(t, uint32(100+len(data)), c.seqNum)
}

func TestConn_Close(t *testing.T) {
	mockConn := NewMockPacketConn()
	remoteAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1").To4(), Port: 12345}
	c := NewConn(mockConn, remoteAddr)

	// Set to ESTABLISHED
	c.state.ProcessEvent(tcpconn.PASSIVE_OPEN)
	c.state.ProcessEvent(tcpconn.SYN)
	c.state.ProcessEvent(tcpconn.ACK)

	err := c.Close()
	require.NoError(t, err)
	require.True(t, c.closed)

	// Second close should be idempotent
	err = c.Close()
	require.NoError(t, err)
}

func TestConn_ReadAfterClose(t *testing.T) {
	mockConn := NewMockPacketConn()
	remoteAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1").To4(), Port: 12345}
	c := NewConn(mockConn, remoteAddr)

	c.Close()

	buf := make([]byte, 1024)
	_, err := c.Read(buf)
	require.Error(t, err)
	require.Equal(t, net.ErrClosed, err)
}

func TestConn_WriteAfterClose(t *testing.T) {
	mockConn := NewMockPacketConn()
	remoteAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1").To4(), Port: 12345}
	c := NewConn(mockConn, remoteAddr)

	c.Close()

	_, err := c.Write([]byte("test"))
	require.Error(t, err)
	require.Equal(t, net.ErrClosed, err)
}

func TestConn_OutOfOrderPackets(t *testing.T) {
	mockConn := NewMockPacketConn()
	remoteAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1").To4(), Port: 12345}
	c := NewConn(mockConn, remoteAddr)
	defer c.Close()

	// Set to ESTABLISHED
	c.state.ProcessEvent(tcpconn.PASSIVE_OPEN)
	c.state.ProcessEvent(tcpconn.SYN)
	c.state.ProcessEvent(tcpconn.ACK)

	c.ackNum = 100

	// Receive packet 2 first (out of order)
	pkt2 := NewPacket(12345, 8080, 105, 0, false, true, false, false, 4096, []byte("World"))
	c.HandlePacket(pkt2)

	// Should be buffered, not delivered yet
	require.Equal(t, uint32(100), c.ackNum)
	require.Equal(t, 0, c.readBuffer.Available())

	// Now receive packet 1
	pkt1 := NewPacket(12345, 8080, 100, 0, false, true, false, false, 4096, []byte("Hello"))
	c.HandlePacket(pkt1)

	// Both packets should be delivered in order
	require.Equal(t, uint32(110), c.ackNum)
	require.Equal(t, 10, c.readBuffer.Available())

	buf := make([]byte, 1024)
	n, _ := c.readBuffer.Read(buf)
	require.Equal(t, "HelloWorld", string(buf[:n]))
}
