package tcpv2

import (
	"net"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/stretchr/testify/require"
)

func TestNewPacket(t *testing.T) {
	pkt := NewPacket(12345, 80, 1000, 2000, true, true, false, false, 4096, []byte("test"))
	
	require.NotNil(t, pkt)
	require.NotNil(t, pkt.TCP)
	require.Equal(t, layers.TCPPort(12345), pkt.TCP.SrcPort)
	require.Equal(t, layers.TCPPort(80), pkt.TCP.DstPort)
	require.Equal(t, uint32(1000), pkt.TCP.Seq)
	require.Equal(t, uint32(2000), pkt.TCP.Ack)
	require.True(t, pkt.TCP.SYN)
	require.True(t, pkt.TCP.ACK)
	require.False(t, pkt.TCP.FIN)
	require.False(t, pkt.TCP.RST)
	require.Equal(t, uint16(4096), pkt.TCP.Window)
	require.Equal(t, []byte("test"), pkt.Payload)
}

func TestPacketEncoding_GopacketCompatibility(t *testing.T) {
	// Create our packet
	pkt := NewPacket(12345, 80, 1000, 2000, true, true, false, false, 4096, []byte("Hello World"))

	srcIP := net.ParseIP("192.168.1.1").To4()
	dstIP := net.ParseIP("192.168.1.2").To4()

	// Encode it
	raw, err := pkt.Encode(srcIP, dstIP)
	require.NoError(t, err)

	// Decode with gopacket
	packet := gopacket.NewPacket(raw, layers.LayerTypeTCP, gopacket.Default)
	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	require.NotNil(t, tcpLayer, "Should be a valid TCP packet")

	tcp, ok := tcpLayer.(*layers.TCP)
	require.True(t, ok)

	// Verify fields
	require.Equal(t, layers.TCPPort(12345), tcp.SrcPort)
	require.Equal(t, layers.TCPPort(80), tcp.DstPort)
	require.Equal(t, uint32(1000), tcp.Seq)
	require.Equal(t, uint32(2000), tcp.Ack)
	require.True(t, tcp.SYN)
	require.True(t, tcp.ACK)
	require.False(t, tcp.FIN)
	require.Equal(t, uint16(4096), tcp.Window)
	require.Equal(t, []byte("Hello World"), tcp.Payload)
}

func TestChecksumCalculation(t *testing.T) {
	// Create a packet
	pkt := NewPacket(12345, 80, 1, 0, true, false, false, false, 1024, []byte{})

	srcIP := net.ParseIP("127.0.0.1").To4()
	dstIP := net.ParseIP("127.0.0.1").To4()

	// Encode with checksum
	raw, err := pkt.Encode(srcIP, dstIP)
	require.NoError(t, err)

	// Decode and verify checksum was set
	packet := gopacket.NewPacket(raw, layers.LayerTypeTCP, gopacket.Default)
	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	require.NotNil(t, tcpLayer)

	tcp, _ := tcpLayer.(*layers.TCP)
	require.NotZero(t, tcp.Checksum, "Checksum should be calculated")
}

func TestDecodePacket(t *testing.T) {
	// Create a valid TCP packet using gopacket
	tcp := &layers.TCP{
		SrcPort:    layers.TCPPort(5555),
		DstPort:    layers.TCPPort(8080),
		Seq:        123456,
		Ack:        654321,
		PSH:        true,
		ACK:        true,
		Window:     8192,
		DataOffset: 5,
	}
	payload := []byte("Test Payload")

	buffer := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true}
	err := gopacket.SerializeLayers(buffer, opts, tcp, gopacket.Payload(payload))
	require.NoError(t, err)

	raw := buffer.Bytes()

	// Decode using our library
	pkt, err := DecodePacket(raw)
	require.NoError(t, err)

	require.Equal(t, layers.TCPPort(5555), pkt.TCP.SrcPort)
	require.Equal(t, layers.TCPPort(8080), pkt.TCP.DstPort)
	require.Equal(t, uint32(123456), pkt.TCP.Seq)
	require.Equal(t, uint32(654321), pkt.TCP.Ack)
	require.True(t, pkt.TCP.PSH)
	require.True(t, pkt.TCP.ACK)
	require.Equal(t, uint16(8192), pkt.TCP.Window)
	require.Equal(t, payload, pkt.Payload)
}


func TestPacketString(t *testing.T) {
	pkt := NewPacket(12345, 80, 1000, 2000, true, true, false, false, 4096, []byte("test"))
	
	str := pkt.String()
	require.Contains(t, str, "Seq=1000")
	require.Contains(t, str, "Ack=2000")
	require.Contains(t, str, "SYN")
	require.Contains(t, str, "ACK")
	require.Contains(t, str, "Src=12345")
	require.Contains(t, str, "Dst=80")
}

func TestPacketEncoding_AllFlags(t *testing.T) {
	tests := []struct {
		name string
		syn  bool
		ack  bool
		fin  bool
		rst  bool
	}{
		{"SYN only", true, false, false, false},
		{"SYN-ACK", true, true, false, false},
		{"ACK only", false, true, false, false},
		{"FIN-ACK", false, true, true, false},
		{"RST", false, false, false, true},
	}

	srcIP := net.ParseIP("10.0.0.1").To4()
	dstIP := net.ParseIP("10.0.0.2").To4()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt := NewPacket(1234, 5678, 100, 200, tt.syn, tt.ack, tt.fin, tt.rst, 1024, nil)
			
			raw, err := pkt.Encode(srcIP, dstIP)
			require.NoError(t, err)
			
			decoded, err := DecodePacket(raw)
			require.NoError(t, err)
			require.Equal(t, tt.syn, decoded.TCP.SYN)
			require.Equal(t, tt.ack, decoded.TCP.ACK)
			require.Equal(t, tt.fin, decoded.TCP.FIN)
			require.Equal(t, tt.rst, decoded.TCP.RST)
		})
	}
}
