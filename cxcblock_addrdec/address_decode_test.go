package cxcblock_addrdec

import (
	"encoding/hex"
	"testing"
)

func TestAddressDecoder_AddressEncode(t *testing.T) {
	Default.IsTestNet = false

	p2pk, _ := hex.DecodeString("edcf627f81c4d50e1c7c0b3339c02d051559246d")
	p2pkAddr, _ := Default.AddressEncode(p2pk)
	t.Logf("p2pkAddr: %s", p2pkAddr)

	//p2sh, _ := hex.DecodeString("131a861f0609944596e2d618e41ba8ce07b281d0")
	//p2shAddr, _ := Default.AddressEncode(p2sh, CXC_mainnetAddressP2SH)
	//t.Logf("p2shAddr: %s", p2shAddr)
}

func TestAddressDecoder_AddressDecode(t *testing.T) {

	Default.IsTestNet = false

	p2pkAddr := "1DNLAJKdPUVUZrrQgJhebfkz7j4bsHroiS"
	p2pkHash, _ := Default.AddressDecode(p2pkAddr)
	t.Logf("p2pkHash: %s", hex.EncodeToString(p2pkHash))

	//p2shAddr := "sQMG5PncvvxVMrVwXpFfBoi3JFHvPiA9aw"
	//
	//p2shHash, _ := vollar_addrdec.Default.AddressDecode(p2shAddr, vollar_addrdec.VDS_mainnetAddressP2SH)
	//t.Logf("p2shHash: %s", hex.EncodeToString(p2shHash))
}
