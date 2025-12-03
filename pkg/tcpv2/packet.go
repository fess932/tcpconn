package tcpv2

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	// HeaderSize is the size of the packet header in bytes
	HeaderSize = 13 // 4 (Seq) + 4 (Ack) + 1 (Flags) + 2 (Win) + 2 (DataLen)
)

// Flags
const (
	FlagSYN uint8 = 1 << iota
	FlagACK
	FlagFIN
	FlagRST
)

// PacketHeader represents the header of our TCP-over-UDP packet
type PacketHeader struct {
	SeqNum     uint32
	AckNum     uint32
	Flags      uint8
	WindowSize uint16
	DataLen    uint16
}

// Packet represents a complete packet
type Packet struct {
	Header PacketHeader
	Data   []byte
}

// Encode encodes the packet into a byte slice
func (p *Packet) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)

	// Write Header
	if err := binary.Write(buf, binary.BigEndian, p.Header.SeqNum); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, p.Header.AckNum); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, p.Header.Flags); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, p.Header.WindowSize); err != nil {
		return nil, err
	}
	// We calculate DataLen automatically, but for safety we write what's in header if 0, or update it
	if p.Header.DataLen == 0 && len(p.Data) > 0 {
		p.Header.DataLen = uint16(len(p.Data))
	}
	if err := binary.Write(buf, binary.BigEndian, p.Header.DataLen); err != nil {
		return nil, err
	}

	// Write Data
	if len(p.Data) > 0 {
		if _, err := buf.Write(p.Data); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// DecodePacket decodes a byte slice into a Packet
func DecodePacket(data []byte) (*Packet, error) {
	if len(data) < HeaderSize {
		return nil, errors.New("packet too short")
	}

	buf := bytes.NewReader(data)
	var h PacketHeader

	if err := binary.Read(buf, binary.BigEndian, &h.SeqNum); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &h.AckNum); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &h.Flags); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &h.WindowSize); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &h.DataLen); err != nil {
		return nil, err
	}

	// Check if declared length matches actual remaining data
	remaining := buf.Len()
	if int(h.DataLen) > remaining {
		return nil, fmt.Errorf("payload length mismatch: header says %d, got %d", h.DataLen, remaining)
	}

	payload := make([]byte, h.DataLen)
	if h.DataLen > 0 {
		if _, err := buf.Read(payload); err != nil {
			return nil, err
		}
	}

	return &Packet{
		Header: h,
		Data:   payload,
	}, nil
}

// String returns a string representation of the packet for debugging
func (p *Packet) String() string {
	var flags []string
	if p.Header.Flags&FlagSYN != 0 {
		flags = append(flags, "SYN")
	}
	if p.Header.Flags&FlagACK != 0 {
		flags = append(flags, "ACK")
	}
	if p.Header.Flags&FlagFIN != 0 {
		flags = append(flags, "FIN")
	}
	if p.Header.Flags&FlagRST != 0 {
		flags = append(flags, "RST")
	}
	if len(flags) == 0 {
		flags = append(flags, "NONE")
	}

	return fmt.Sprintf("Seq=%d Ack=%d Flags=%v Win=%d Len=%d",
		p.Header.SeqNum, p.Header.AckNum, flags, p.Header.WindowSize, p.Header.DataLen)
}
