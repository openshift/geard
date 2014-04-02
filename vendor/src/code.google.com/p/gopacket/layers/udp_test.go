// Copyright 2012, Google, Inc. All rights reserved.
// Copyright 2009-2011 Andreas Krennmair. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"code.google.com/p/gopacket"
	"reflect"
	"testing"
)

// testUDPPacketDNS is the packet:
//   10:33:07.883637 IP 172.16.255.1.53 > 172.29.20.15.35181: 47320 7/0/0 MX ALT2.ASPMX.L.GOOGLE.com. 20, MX ASPMX2.GOOGLEMAIL.com. 30, MX ASPMX3.GOOGLEMAIL.com. 30, MX ASPMX4.GOOGLEMAIL.com. 30, MX ASPMX5.GOOGLEMAIL.com. 30, MX ASPMX.L.GOOGLE.com. 10, MX ALT1.ASPMX.L.GOOGLE.com. 20 (202)
//      0x0000:  24be 0527 0b17 001f cab3 75c0 0800 4500  $..'......u...E.
//      0x0010:  00e6 68cf 0000 3f11 a6f9 ac10 ff01 ac1d  ..h...?.........
//      0x0020:  140f 0035 896d 00d2 754a b8d8 8180 0001  ...5.m..uJ......
//      0x0030:  0007 0000 0000 0478 6b63 6403 636f 6d00  .......xkcd.com.
//      0x0040:  000f 0001 c00c 000f 0001 0000 0258 0018  .............X..
//      0x0050:  0014 0441 4c54 3205 4153 504d 5801 4c06  ...ALT2.ASPMX.L.
//      0x0060:  474f 4f47 4c45 c011 c00c 000f 0001 0000  GOOGLE..........
//      0x0070:  0258 0016 001e 0641 5350 4d58 320a 474f  .X.....ASPMX2.GO
//      0x0080:  4f47 4c45 4d41 494c c011 c00c 000f 0001  OGLEMAIL........
//      0x0090:  0000 0258 000b 001e 0641 5350 4d58 33c0  ...X.....ASPMX3.
//      0x00a0:  53c0 0c00 0f00 0100 0002 5800 0b00 1e06  S.........X.....
//      0x00b0:  4153 504d 5834 c053 c00c 000f 0001 0000  ASPMX4.S........
//      0x00c0:  0258 000b 001e 0641 5350 4d58 35c0 53c0  .X.....ASPMX5.S.
//      0x00d0:  0c00 0f00 0100 0002 5800 0400 0ac0 2dc0  ........X.....-.
//      0x00e0:  0c00 0f00 0100 0002 5800 0900 1404 414c  ........X.....AL
//      0x00f0:  5431 c02d                                T1.-
// Packet generated by doing DNS query for 'xkcd.com'
var testUDPPacketDNS = []byte{
	0x24, 0xbe, 0x05, 0x27, 0x0b, 0x17, 0x00, 0x1f, 0xca, 0xb3, 0x75, 0xc0, 0x08, 0x00, 0x45, 0x00,
	0x00, 0xe6, 0x68, 0xcf, 0x00, 0x00, 0x3f, 0x11, 0xa6, 0xf9, 0xac, 0x10, 0xff, 0x01, 0xac, 0x1d,
	0x14, 0x0f, 0x00, 0x35, 0x89, 0x6d, 0x00, 0xd2, 0x75, 0x4a, 0xb8, 0xd8, 0x81, 0x80, 0x00, 0x01,
	0x00, 0x07, 0x00, 0x00, 0x00, 0x00, 0x04, 0x78, 0x6b, 0x63, 0x64, 0x03, 0x63, 0x6f, 0x6d, 0x00,
	0x00, 0x0f, 0x00, 0x01, 0xc0, 0x0c, 0x00, 0x0f, 0x00, 0x01, 0x00, 0x00, 0x02, 0x58, 0x00, 0x18,
	0x00, 0x14, 0x04, 0x41, 0x4c, 0x54, 0x32, 0x05, 0x41, 0x53, 0x50, 0x4d, 0x58, 0x01, 0x4c, 0x06,
	0x47, 0x4f, 0x4f, 0x47, 0x4c, 0x45, 0xc0, 0x11, 0xc0, 0x0c, 0x00, 0x0f, 0x00, 0x01, 0x00, 0x00,
	0x02, 0x58, 0x00, 0x16, 0x00, 0x1e, 0x06, 0x41, 0x53, 0x50, 0x4d, 0x58, 0x32, 0x0a, 0x47, 0x4f,
	0x4f, 0x47, 0x4c, 0x45, 0x4d, 0x41, 0x49, 0x4c, 0xc0, 0x11, 0xc0, 0x0c, 0x00, 0x0f, 0x00, 0x01,
	0x00, 0x00, 0x02, 0x58, 0x00, 0x0b, 0x00, 0x1e, 0x06, 0x41, 0x53, 0x50, 0x4d, 0x58, 0x33, 0xc0,
	0x53, 0xc0, 0x0c, 0x00, 0x0f, 0x00, 0x01, 0x00, 0x00, 0x02, 0x58, 0x00, 0x0b, 0x00, 0x1e, 0x06,
	0x41, 0x53, 0x50, 0x4d, 0x58, 0x34, 0xc0, 0x53, 0xc0, 0x0c, 0x00, 0x0f, 0x00, 0x01, 0x00, 0x00,
	0x02, 0x58, 0x00, 0x0b, 0x00, 0x1e, 0x06, 0x41, 0x53, 0x50, 0x4d, 0x58, 0x35, 0xc0, 0x53, 0xc0,
	0x0c, 0x00, 0x0f, 0x00, 0x01, 0x00, 0x00, 0x02, 0x58, 0x00, 0x04, 0x00, 0x0a, 0xc0, 0x2d, 0xc0,
	0x0c, 0x00, 0x0f, 0x00, 0x01, 0x00, 0x00, 0x02, 0x58, 0x00, 0x09, 0x00, 0x14, 0x04, 0x41, 0x4c,
	0x54, 0x31, 0xc0, 0x2d,
}

func TestUDPPacketDNS(t *testing.T) {
	p := gopacket.NewPacket(testUDPPacketDNS, LinkTypeEthernet, gopacket.Default)
	if p.ErrorLayer() != nil {
		t.Error("Failed to decode packet:", p.ErrorLayer().Error())
	}
	checkLayers(p, []gopacket.LayerType{LayerTypeEthernet, LayerTypeIPv4, LayerTypeUDP, gopacket.LayerTypePayload}, t)
	if got, ok := p.TransportLayer().(*UDP); ok {
		want := &UDP{
			BaseLayer: BaseLayer{
				Contents: []byte{0x0, 0x35, 0x89, 0x6d, 0x0, 0xd2, 0x75, 0x4a},
				Payload: []byte{0xb8, 0xd8, 0x81, 0x80, 0x0, 0x1, 0x0,
					0x7, 0x0, 0x0, 0x0, 0x0, 0x4, 0x78, 0x6b, 0x63, 0x64, 0x3, 0x63, 0x6f,
					0x6d, 0x0, 0x0, 0xf, 0x0, 0x1, 0xc0, 0xc, 0x0, 0xf, 0x0, 0x1, 0x0, 0x0,
					0x2, 0x58, 0x0, 0x18, 0x0, 0x14, 0x4, 0x41, 0x4c, 0x54, 0x32, 0x5, 0x41,
					0x53, 0x50, 0x4d, 0x58, 0x1, 0x4c, 0x6, 0x47, 0x4f, 0x4f, 0x47, 0x4c,
					0x45, 0xc0, 0x11, 0xc0, 0xc, 0x0, 0xf, 0x0, 0x1, 0x0, 0x0, 0x2, 0x58, 0x0,
					0x16, 0x0, 0x1e, 0x6, 0x41, 0x53, 0x50, 0x4d, 0x58, 0x32, 0xa, 0x47, 0x4f,
					0x4f, 0x47, 0x4c, 0x45, 0x4d, 0x41, 0x49, 0x4c, 0xc0, 0x11, 0xc0, 0xc,
					0x0, 0xf, 0x0, 0x1, 0x0, 0x0, 0x2, 0x58, 0x0, 0xb, 0x0, 0x1e, 0x6, 0x41,
					0x53, 0x50, 0x4d, 0x58, 0x33, 0xc0, 0x53, 0xc0, 0xc, 0x0, 0xf, 0x0, 0x1,
					0x0, 0x0, 0x2, 0x58, 0x0, 0xb, 0x0, 0x1e, 0x6, 0x41, 0x53, 0x50, 0x4d,
					0x58, 0x34, 0xc0, 0x53, 0xc0, 0xc, 0x0, 0xf, 0x0, 0x1, 0x0, 0x0, 0x2,
					0x58, 0x0, 0xb, 0x0, 0x1e, 0x6, 0x41, 0x53, 0x50, 0x4d, 0x58, 0x35, 0xc0,
					0x53, 0xc0, 0xc, 0x0, 0xf, 0x0, 0x1, 0x0, 0x0, 0x2, 0x58, 0x0, 0x4, 0x0,
					0xa, 0xc0, 0x2d, 0xc0, 0xc, 0x0, 0xf, 0x0, 0x1, 0x0, 0x0, 0x2, 0x58, 0x0,
					0x9, 0x0, 0x14, 0x4, 0x41, 0x4c, 0x54, 0x31, 0xc0, 0x2d},
			},
			SrcPort:  53,
			DstPort:  35181,
			Length:   210,
			Checksum: 30026,
			sPort:    []byte{0x0, 0x35},
			dPort:    []byte{0x89, 0x6d},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("UDP packet mismatch:\ngot  :\n%#v\n\nwant :\n%#v\n\n", got, want)
		}
	} else {
		t.Error("Transport layer packet not UDP")
	}
}
