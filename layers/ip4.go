// Copyright 2012 Google, Inc. All rights reserved.
// Copyright 2009-2011 Andreas Krennmair. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"code.google.com/p/gopacket"
	"encoding/binary"
	"fmt"
	"net"
)

// IPv4 is the header of an IP packet.
type IPv4 struct {
	BaseLayer
	Version    uint8
	IHL        uint8
	TOS        uint8
	Length     uint16
	Id         uint16
	Flags      uint8
	FragOffset uint16
	TTL        uint8
	Protocol   IPProtocol
	Checksum   uint16
	SrcIP      net.IP
	DstIP      net.IP
	Options    []IPv4Option
	Padding    []byte
}

// LayerType returns LayerTypeIPv4
func (i *IPv4) LayerType() gopacket.LayerType { return LayerTypeIPv4 }
func (i *IPv4) NetworkFlow() gopacket.Flow {
	return gopacket.NewFlow(EndpointIPv4, i.SrcIP, i.DstIP)
}

type IPv4Option struct {
	OptionType   uint8
	OptionLength uint8
	OptionData   []byte
}

func (i IPv4Option) String() string {
	return fmt.Sprintf("IPv4Option(%v:%v)", i.OptionType, i.OptionData)
}

func (ip *IPv4) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	flagsfrags := binary.BigEndian.Uint16(data[6:8])

	ip.Version = uint8(data[0]) >> 4
	ip.IHL = uint8(data[0]) & 0x0F
	ip.TOS = data[1]
	ip.Length = binary.BigEndian.Uint16(data[2:4])
	ip.Id = binary.BigEndian.Uint16(data[4:6])
	ip.Flags = uint8(flagsfrags >> 13)
	ip.FragOffset = flagsfrags & 0x1FFF
	ip.TTL = data[8]
	ip.Protocol = IPProtocol(data[9])
	ip.Checksum = binary.BigEndian.Uint16(data[10:12])
	ip.SrcIP = data[12:16]
	ip.DstIP = data[16:20]
	// Set up an initial guess for contents/payload... we'll reset these soon.
	ip.BaseLayer = BaseLayer{Contents: data}

	if ip.Length < 20 {
		return fmt.Errorf("Invalid (too small) IP length (%d < 20)", ip.Length)
	} else if ip.IHL < 5 {
		return fmt.Errorf("Invalid (too small) IP header length (%d < 5)", ip.IHL)
	} else if int(ip.IHL*4) > int(ip.Length) {
		return fmt.Errorf("Invalid IP header length > IP length (%d > %d)", ip.IHL, ip.Length)
	}
	if cmp := len(data) - int(ip.Length); cmp > 0 {
		data = data[:ip.Length]
	} else if cmp < 0 {
		df.SetTruncated()
		if int(ip.IHL)*4 > len(data) {
			return fmt.Errorf("Not all IP header bytes available")
		}
	}
	ip.Contents = data[:ip.IHL*4]
	ip.Payload = data[ip.IHL*4:]
	// From here on, data contains the header options.
	data = data[20 : ip.IHL*4]
	// Pull out IP options
	for len(data) > 0 {
		if ip.Options == nil {
			// Pre-allocate to avoid growing the slice too much.
			ip.Options = make([]IPv4Option, 0, 4)
		}
		opt := IPv4Option{OptionType: data[0]}
		switch opt.OptionType {
		case 0: // End of options
			opt.OptionLength = 1
			ip.Options = append(ip.Options, opt)
			ip.Padding = data[1:]
			break
		case 1: // 1 byte padding
			opt.OptionLength = 1
		default:
			opt.OptionLength = data[1]
			opt.OptionData = data[2:opt.OptionLength]
		}
		if len(data) >= int(opt.OptionLength) {
			data = data[opt.OptionLength:]
		} else {
			return fmt.Errorf("IP option length exceeds remaining IP header size, option type %v length %v", opt.OptionType, opt.OptionLength)
		}
		ip.Options = append(ip.Options, opt)
	}
	return nil
}

func (i *IPv4) CanDecode() gopacket.LayerClass {
	return LayerTypeIPv4
}

func (i *IPv4) NextLayerType() gopacket.LayerType {
	return i.Protocol.LayerType()
}

func decodeIPv4(data []byte, p gopacket.PacketBuilder) error {
	ip := &IPv4{}
	err := ip.DecodeFromBytes(data, p)
	p.AddLayer(ip)
	p.SetNetworkLayer(ip)
	if err != nil {
		return err
	}
	return p.NextDecoder(ip.Protocol)
}