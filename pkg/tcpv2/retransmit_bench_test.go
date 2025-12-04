package tcpv2

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"tcpconn"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// BenchmarkRetransmission_WithPacketLoss тестирует производительность ретрансмиссии
// при различных уровнях потери пакетов
func BenchmarkRetransmission_WithPacketLoss(b *testing.B) {
	// Различные уровни потери пакетов
	lossRates := []struct {
		name       string
		dropEveryN int32
	}{
		{"NoLoss", 0},      // 0% потерь
		{"Loss_10pct", 10}, // ~10% потерь
		{"Loss_20pct", 5},  // ~20% потерь
	}

	// Различные размеры данных
	dataSizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"10KB", 10 * 1024},
	}

	for _, loss := range lossRates {
		for _, dataSize := range dataSizes {
			name := fmt.Sprintf("%s_%s", loss.name, dataSize.name)
			b.Run(name, func(b *testing.B) {
				benchmarkRetransmission(b, loss.dropEveryN, dataSize.size)
			})
		}
	}
}

func benchmarkRetransmission(b *testing.B, dropEveryN int32, dataSize int) {
	// Start server
	serverListener, err := net.ListenPacket("udp4", "127.0.0.1:0")
	require.NoError(b, err)
	defer serverListener.Close()

	serverAddr := serverListener.LocalAddr().String()

	// Server state
	var serverConn *Conn
	serverDone := make(chan struct{})

	// Server goroutine
	go func() {
		defer close(serverDone)

		// Accept connection
		buf := make([]byte, 65535)
		n, clientAddr, err := serverListener.ReadFrom(buf)
		require.NoError(b, err)

		// Decode SYN
		synPkt, err := DecodePacket(buf[:n])
		if err != nil || !synPkt.TCP.SYN {
			return
		}

		// Create server connection
		serverConn = NewConn(serverListener, clientAddr)
		serverConn.state.ProcessEvent(tcpconn.PASSIVE_OPEN)
		serverConn.ackNum = synPkt.TCP.Seq + 1

		// Send SYN-ACK
		serverConn.sendControlPacket(true, true, false, false)

		// Read incoming packets in background
		go func() {
			buf := make([]byte, DefaultWindowSize)
			for {
				n, addr, err := serverListener.ReadFrom(buf)
				if err != nil {
					// log.Error().Err(err).Msg("Error reading from server listener")
					return
				}
				if addr.String() != clientAddr.String() {
					continue
				}
				pkt, err := DecodePacket(buf[:n])
				require.NoError(b, err)
				serverConn.HandlePacket(pkt)
			}
		}()

		// Echo server loop
		dataBuf := make([]byte, DefaultWindowSize)
		for {
			n, err := serverConn.Read(dataBuf)
			require.NoError(b, err)
			if n > 0 {
				_, err = serverConn.Write(dataBuf[:n])
				require.NoError(b, err)
			}
		}
	}()

	// Client with lossy connection
	clientConn, err := net.ListenPacket("udp4", ":0")
	require.NoError(b, err)
	defer clientConn.Close()

	lossyConn := NewLossyPacketConn(clientConn, dropEveryN)

	raddr, err := net.ResolveUDPAddr("udp4", serverAddr)
	require.NoError(b, err)

	client := NewConn(lossyConn, raddr)
	defer client.Close()

	// Read loop
	go func() {
		buf := make([]byte, 65535)
		for {
			n, addr, err := clientConn.ReadFrom(buf) // сначала запускать соединение потом начинать чтение может быть?
			if err != nil {
				return
			}
			if addr.String() != raddr.String() {
				continue
			}
			pkt, err := DecodePacket(buf[:n])
			require.NoError(b, err)
			client.HandlePacket(pkt)
		}
	}()

	// Initiate connection
	err = client.state.ProcessEvent(tcpconn.ACTIVE_OPEN)
	require.NoError(b, err)

	client.seqNum = 100
	err = client.sendControlPacket(true, false, false, false)
	require.NoError(b, err)

	<-client.connected

	require.Truef(b, client.state.IsConnected(), "Connection should be established")

	// Prepare test data
	testData := []byte(strings.Repeat("X", dataSize))

	// Reset timer before actual benchmark
	b.ResetTimer()

	// Run benchmark
	for i := 0; i < b.N; i++ {
		// Send data
		n, err := client.Write(testData)
		require.NoError(b, err)
		require.Lenf(b, testData, n, "Expected to write %d bytes, wrote %d", len(testData), n)

		// Read echo
		buf := make([]byte, len(testData))
		totalRead := 0
		for totalRead < len(testData) {
			n, err := client.Read(buf[totalRead:])
			require.NoError(b, err)
			totalRead += n
		}
		require.Lenf(b, testData, totalRead, "Expected to read %d bytes, read %d", len(testData), totalRead)
	}

	b.StopTimer()

	// Report statistics
	dropped, total := lossyConn.GetStats()
	if total > 0 {
		lossRate := float64(dropped) / float64(total) * 100
		b.ReportMetric(lossRate, "loss_%")
		b.ReportMetric(float64(dropped), "dropped_packets")
		b.ReportMetric(float64(total), "total_packets")
	}

	// Calculate throughput
	bytesTransferred := int64(dataSize * b.N * 2) // *2 for send+receive
	b.SetBytes(int64(dataSize))
	b.ReportMetric(float64(bytesTransferred)/b.Elapsed().Seconds()/1024/1024, "MB/s")
}

// BenchmarkRetransmission_Concurrent тестирует производительность
// при одновременной работе нескольких соединений с потерей пакетов
func BenchmarkRetransmission_Concurrent(b *testing.B) {
	concurrencyLevels := []int{1, 2, 4}
	dropEveryN := int32(10) // ~10% потерь
	dataSize := 2 * 1024    // 2KB

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Conns_%d", concurrency), func(b *testing.B) {
			benchmarkConcurrentRetransmission(b, concurrency, dropEveryN, dataSize)
		})
	}
}

func benchmarkConcurrentRetransmission(b *testing.B, concurrency int, dropEveryN int32, dataSize int) {
	// Start server
	serverListener, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer serverListener.Close()

	serverAddr := serverListener.LocalAddr().String()

	// Server connections map
	serverConns := sync.Map{}
	serverReady := make(chan struct{})

	// Server goroutine - handles multiple clients
	go func() {
		// Background packet reader
		go func() {
			buf := make([]byte, 65535)
			for {
				n, addr, err := serverListener.ReadFrom(buf)
				if err != nil {
					return
				}

				addrStr := addr.String()
				pkt, err := DecodePacket(buf[:n])
				if err != nil {
					continue
				}

				// Check if we have a connection for this client
				if connVal, ok := serverConns.Load(addrStr); ok {
					serverConn := connVal.(*Conn)
					serverConn.HandlePacket(pkt)
				} else if pkt.TCP.SYN {
					// New connection
					serverConn := NewConn(serverListener, addr)
					serverConn.state.ProcessEvent(tcpconn.PASSIVE_OPEN)
					serverConn.ackNum = pkt.TCP.Seq + 1
					serverConns.Store(addrStr, serverConn)

					// Send SYN-ACK
					serverConn.sendControlPacket(true, true, false, false)

					// Start echo server for this connection
					go func(conn *Conn) {
						dataBuf := make([]byte, DefaultWindowSize)
						for {
							n, err := conn.Read(dataBuf)
							if err != nil {
								return
							}
							if n > 0 {
								conn.Write(dataBuf[:n])
							}
						}
					}(serverConn)
				}
			}
		}()

		close(serverReady)
	}()

	<-serverReady
	time.Sleep(100 * time.Millisecond)

	// Create clients
	clients := make([]*Conn, concurrency)
	clientConns := make([]net.PacketConn, concurrency)

	for i := 0; i < concurrency; i++ {
		clientConn, err := net.ListenPacket("udp4", ":0")
		if err != nil {
			b.Fatal(err)
		}
		defer clientConn.Close()
		clientConns[i] = clientConn

		lossyConn := NewLossyPacketConn(clientConn, dropEveryN)

		raddr, err := net.ResolveUDPAddr("udp4", serverAddr)
		if err != nil {
			b.Fatal(err)
		}

		client := NewConn(lossyConn, raddr)
		clients[i] = client
		defer client.Close()

		// Read loop for this client
		go func(idx int) {
			buf := make([]byte, 65535)
			for {
				n, addr, err := clientConns[idx].ReadFrom(buf)
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
				clients[idx].HandlePacket(pkt)
			}
		}(i)

		// Initiate connection
		client.state.ProcessEvent(tcpconn.ACTIVE_OPEN)
		client.seqNum = uint32(1000 + i*1000)
		client.sendControlPacket(true, false, false, false)
	}

	// Wait for all connections to establish
	time.Sleep(500 * time.Millisecond)

	// Verify all connections
	for i, client := range clients {
		if !client.state.IsConnected() {
			b.Fatalf("Client %d failed to connect", i)
		}
	}

	// Prepare test data
	testData := []byte(strings.Repeat("X", dataSize))

	b.ResetTimer()

	// Run benchmark with concurrent clients
	b.RunParallel(func(pb *testing.PB) {
		clientIdx := 0
		for pb.Next() {
			client := clients[clientIdx%concurrency]
			clientIdx++

			// Send data
			n, err := client.Write(testData)
			if err != nil {
				b.Error(err)
				continue
			}
			if n != len(testData) {
				b.Errorf("Expected to write %d bytes, wrote %d", len(testData), n)
				continue
			}

			// Read echo
			buf := make([]byte, len(testData))
			totalRead := 0
			for totalRead < len(testData) {
				n, err := client.Read(buf[totalRead:])
				if err != nil {
					b.Error(err)
					break
				}
				totalRead += n
			}
		}
	})

	b.StopTimer()

	// Report aggregate statistics
	var totalDropped, totalPackets int64
	for i := 0; i < concurrency; i++ {
		if lossyConn, ok := clients[i].conn.(*LossyPacketConn); ok {
			dropped, total := lossyConn.GetStats()
			totalDropped += int64(dropped)
			totalPackets += int64(total)
		}
	}

	if totalPackets > 0 {
		lossRate := float64(totalDropped) / float64(totalPackets) * 100
		b.ReportMetric(lossRate, "loss_%")
		b.ReportMetric(float64(totalDropped), "dropped_packets")
		b.ReportMetric(float64(totalPackets), "total_packets")
	}

	b.SetBytes(int64(dataSize))
}

// BenchmarkRTO_Adaptation тестирует производительность адаптации RTO
func BenchmarkRTO_Adaptation(b *testing.B) {
	mockConn := NewMockPacketConn()
	remoteAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1").To4(), Port: 12345}
	c := NewConn(mockConn, remoteAddr)
	defer c.Close()

	// Set to ESTABLISHED
	c.state.ProcessEvent(tcpconn.PASSIVE_OPEN)
	c.state.ProcessEvent(tcpconn.SYN)
	c.state.ProcessEvent(tcpconn.ACK)

	rtts := []time.Duration{
		50 * time.Millisecond,
		100 * time.Millisecond,
		75 * time.Millisecond,
		120 * time.Millisecond,
		60 * time.Millisecond,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rtt := rtts[i%len(rtts)]
		c.updateRTO(rtt)
	}
}
