package tcpv2

import (
	"fmt"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// Packet represents a TCP packet
type Packet struct {
	TCP     *layers.TCP
	Payload []byte
}

// NewPacket creates a new packet with the given parameters
func NewPacket(srcPort, dstPort uint16, seq, ack uint32, syn, ack_flag, fin, rst bool, window uint16, data []byte) *Packet {
	tcp := &layers.TCP{
		SrcPort:    layers.TCPPort(srcPort),
		DstPort:    layers.TCPPort(dstPort),
		Seq:        seq,
		Ack:        ack,
		Window:     window,
		DataOffset: 5,
		SYN:        syn,
		ACK:        ack_flag,
		FIN:        fin,
		RST:        rst,
	}

	return &Packet{
		TCP:     tcp,
		Payload: data,
	}
}

// Encode serializes the packet to bytes (IPv4 only)
func (p *Packet) Encode(srcIP, dstIP net.IP) ([]byte, error) {
	// Convert to IPv4
	srcIPv4 := srcIP.To4()
	dstIPv4 := dstIP.To4()
	if srcIPv4 == nil || dstIPv4 == nil {
		return nil, fmt.Errorf("only IPv4 addresses are supported")
	}
	
	// Set network layer for checksum calculation
	if err := p.TCP.SetNetworkLayerForChecksum(&layers.IPv4{
		SrcIP:    srcIPv4,
		DstIP:    dstIPv4,
		Protocol: layers.IPProtocolTCP,
	}); err != nil {
		return nil, fmt.Errorf("failed to set IPv4 network layer for checksum: %w", err)
	}

	buffer := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	var payload gopacket.Payload
	if len(p.Payload) > 0 {
		payload = gopacket.Payload(p.Payload)
		if err := gopacket.SerializeLayers(buffer, opts, p.TCP, payload); err != nil {
			return nil, fmt.Errorf("failed to serialize TCP packet with payload: %w", err)
		}
	} else {
		if err := gopacket.SerializeLayers(buffer, opts, p.TCP); err != nil {
			return nil, fmt.Errorf("failed to serialize TCP packet: %w", err)
		}
	}

	return buffer.Bytes(), nil
}

// DecodePacket decodes a byte slice into a Packet
func DecodePacket(data []byte) (*Packet, error) {
	packet := gopacket.NewPacket(data, layers.LayerTypeTCP, gopacket.Default)
	
	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer == nil {
		return nil, fmt.Errorf("not a valid TCP packet")
	}

	tcp, _ := tcpLayer.(*layers.TCP)
	
	return &Packet{
		TCP:     tcp,
		Payload: tcp.Payload,
	}, nil
}

// String returns a string representation of the packet
func (p *Packet) String() string {
	var flags []string
	if p.TCP.SYN {
		flags = append(flags, "SYN")
	}
	if p.TCP.ACK {
		flags = append(flags, "ACK")
	}
	if p.TCP.FIN {
		flags = append(flags, "FIN")
	}
	if p.TCP.RST {
		flags = append(flags, "RST")
	}
	if len(flags) == 0 {
		flags = append(flags, "NONE")
	}

	return fmt.Sprintf("Seq=%d Ack=%d Flags=%v Win=%d Len=%d Src=%d Dst=%d",
		p.TCP.Seq, p.TCP.Ack, flags, p.TCP.Window, len(p.Payload),
		p.TCP.SrcPort, p.TCP.DstPort)
}
