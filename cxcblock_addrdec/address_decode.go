package cxcblock_addrdec

import (
	"github.com/blocktree/go-owcdrivers/addressEncoder"
)

var (
	alphabet = addressEncoder.BTCAlphabet
)

var (

	CXC_mainnetAddressP2PKH         = addressEncoder.BTC_mainnetAddressP2PKH
	CXC_testnetAddressP2PKH         = addressEncoder.BTC_testnetAddressP2PKH
	CXC_mainnetPrivateWIFCompressed = addressEncoder.BTC_mainnetPrivateWIFCompressed
	CXC_testnetPrivateWIFCompressed = addressEncoder.BTC_testnetPrivateWIFCompressed
	CXC_mainnetAddressP2SH          = addressEncoder.BTC_mainnetAddressP2SH
	CXC_testnetAddressP2SH          = addressEncoder.BTC_testnetAddressP2SH

	Default = AddressDecoderV2{}
)

//AddressDecoderV2
type AddressDecoderV2 struct {
	IsTestNet bool
}

//AddressDecode 地址解析
func (dec *AddressDecoderV2) AddressDecode(addr string, opts ...interface{}) ([]byte, error) {

	cfg := CXC_mainnetAddressP2PKH
	if dec.IsTestNet {
		cfg = CXC_testnetAddressP2PKH
	}

	if len(opts) > 0 {
		for _, opt := range opts {
			if at, ok := opt.(addressEncoder.AddressType); ok {
				cfg = at
			}
		}
	}

	return addressEncoder.AddressDecode(addr, cfg)
}

//AddressEncode 地址编码
func (dec *AddressDecoderV2) AddressEncode(hash []byte, opts ...interface{}) (string, error) {

	cfg := CXC_mainnetAddressP2PKH
	if dec.IsTestNet {
		cfg = CXC_testnetAddressP2PKH
	}

	if len(opts) > 0 {
		for _, opt := range opts {
			if at, ok := opt.(addressEncoder.AddressType); ok {
				cfg = at
			}
		}
	}

	address := addressEncoder.AddressEncode(hash, cfg)
	return address, nil
}
