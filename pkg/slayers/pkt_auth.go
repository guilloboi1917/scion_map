// Copyright 2022 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file includes the SPAO header implementation as specified
// in https://docs.scion.org/en/latest/protocols/authenticator-option.html

// The Authenticator option format is as follows:
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |   NextHdr=UDP |     ExtLen    |  OptType=2    |  OptDataLen   |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                   Security Parameter Index                    |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |    Algorithm  |      RSV      |                               |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+							   |
// |                   Timestamp / Sequence Number                 |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                                                               |
// +                                                               +
// |                                                               |
// +                        16-octet MAC data                      +
// |                                                               |
// +                                                               +
// |                                                               |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

package slayers

import (
	"encoding/binary"

	"github.com/scionproto/scion/pkg/private/serrors"
)

const (
	PacketAuthASHost uint8 = iota
	PacketAuthHostHost
)

const (
	PacketAuthSenderSide uint8 = iota
	PacketAuthReceiverSide
)

const (
	// PacketAuthOptionMetadataLen is the size of the SPAO Metadata and
	// corresponds the minimum size of the SPAO OptData.
	// The SPAO header contains the following fixed-length fields:
	// SPI (4 Bytes), Algorithm (1 Byte), RSV (1 Byte) and
	// Timestamp / Sequence Number (6 Bytes).
	PacketAuthOptionMetadataLen = 12
)

// PacketAuthSPI (Security Parameter Index) is the identifier for the key
// used for the packet authentication option. DRKey values are in the
// range [1, 2^21-1].
type PacketAuthSPI uint32

func (p PacketAuthSPI) Type() uint8 {
	if p&(1<<17) == 0 {
		return PacketAuthASHost
	}
	return PacketAuthHostHost
}

func (p PacketAuthSPI) Direction() uint8 {
	if p&(1<<16) == 0 {
		return PacketAuthSenderSide
	}
	return PacketAuthReceiverSide
}

func (p PacketAuthSPI) DRKeyProto() uint16 {
	return uint16(p)
}

func (p PacketAuthSPI) IsDRKey() bool {
	return p > 0 && p < (1<<21)
}

func MakePacketAuthSPIDRKey(
	proto uint16,
	drkeyType uint8,
	dir uint8,
) (PacketAuthSPI, error) {

	if proto < 1 {
		return 0, serrors.New("Invalid proto identifier value")
	}
	if drkeyType > 1 {
		return 0, serrors.New("Invalid DRKeyType value")
	}
	if dir > 1 {
		return 0, serrors.New("Invalid DRKeyDirection value")
	}
	spi := uint32((drkeyType & 0x1)) << 17
	spi |= uint32((dir & 0x1)) << 16
	spi |= uint32(proto)

	return PacketAuthSPI(spi), nil
}

// PacketAuthAlg is the enumerator for authenticator algorithm types in the
// packet authenticator option.
type PacketAuthAlg uint8

const (
	PacketAuthCMAC PacketAuthAlg = iota
	PacketAuthSHA1_AES_CBC
)

type PacketAuthOptionParams struct {
	SPI         PacketAuthSPI
	Algorithm   PacketAuthAlg
	TimestampSN uint64
	Auth        []byte
}

// PacketAuthOption wraps an EndToEndOption of OptTypeAuthenticator.
// This can be used to serialize and parse the internal structure of the packet authenticator
// option.
type PacketAuthOption struct {
	*EndToEndOption
}

// NewPacketAuthOption creates a new EndToEndOption of
// OptTypeAuthenticator, initialized with the given SPAO data.
func NewPacketAuthOption(
	p PacketAuthOptionParams,
) (PacketAuthOption, error) {

	o := PacketAuthOption{EndToEndOption: new(EndToEndOption)}
	err := o.Reset(p)
	return o, err
}

// ParsePacketAuthOption parses o as a packet authenticator option.
// Performs minimal checks to ensure that SPI, algorithm, timestamp, RSV, and
// sequence number are set.
// Checking the size and content of the Authenticator data must be done by the
// caller.
func ParsePacketAuthOption(o *EndToEndOption) (PacketAuthOption, error) {
	if o.OptType != OptTypeAuthenticator {
		return PacketAuthOption{},
			serrors.New("wrong option type", "expected", OptTypeAuthenticator, "actual", o.OptType)
	}
	if len(o.OptData) < PacketAuthOptionMetadataLen {
		return PacketAuthOption{},
			serrors.New("buffer too short", "expected at least", 12, "actual", len(o.OptData))
	}
	return PacketAuthOption{o}, nil
}

// Reset reinitializes the underlying EndToEndOption with the SPAO data.
// Reuses the OptData buffer if it is of sufficient capacity.
func (o PacketAuthOption) Reset(
	p PacketAuthOptionParams,
) error {

	if p.TimestampSN >= (1 << 48) {
		return serrors.New("Timestamp value should be smaller than 2^48")
	}

	o.OptType = OptTypeAuthenticator

	n := PacketAuthOptionMetadataLen + len(p.Auth)
	if n <= cap(o.OptData) {
		o.OptData = o.OptData[:n]
	} else {
		o.OptData = make([]byte, n)
	}
	binary.BigEndian.PutUint32(o.OptData[:4], uint32(p.SPI))
	o.OptData[4] = byte(p.Algorithm)
	o.OptData[5] = byte(0)
	bigEndianPutUint48(o.OptData[6:12], p.TimestampSN)
	copy(o.OptData[12:], p.Auth)

	o.OptAlign = [2]uint8{4, 2}

	// Set the length field properly. Otherwise the option's data cannot be parsed.
	o.OptDataLen = byte(len(o.OptData))
	o.ActualLength = len(o.OptData) + 2
	return nil
}

// SPI returns the value set in the Security Parameter Index in the extension.
func (o PacketAuthOption) SPI() PacketAuthSPI {
	return PacketAuthSPI(binary.BigEndian.Uint32(o.OptData[:4]))
}

// Algorithm returns the algorithm type stored in the data buffer.
func (o PacketAuthOption) Algorithm() PacketAuthAlg {
	return PacketAuthAlg(o.OptData[4])
}

// Timestamp returns the value set in the homonym field in the extension.
func (o PacketAuthOption) TimestampSN() uint64 {
	return bigEndianUint48(o.OptData[6:12])
}

// Authenticator returns slice of the underlying auth buffer.
// Changes to this slice will be reflected on the wire when
// the extension is serialized.
func (o PacketAuthOption) Authenticator() []byte {
	return o.OptData[12:]
}

func bigEndianUint48(b []byte) uint64 {
	return uint64(b[0])<<40 + uint64(b[1])<<32 +
		uint64(binary.BigEndian.Uint32(b[2:6]))
}

func bigEndianPutUint48(b []byte, v uint64) {
	b[0] = byte(v >> 40)
	b[1] = byte(v >> 32)
	binary.BigEndian.PutUint32(b[2:6], uint32(v))
}
