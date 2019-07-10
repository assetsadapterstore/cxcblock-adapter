/*
 * Copyright 2018 The openwallet Authors
 * This file is part of the openwallet library.
 *
 * The openwallet library is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The openwallet library is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Lesser General Public License for more details.
 */

package cxcblock

import (
	"encoding/hex"
	"github.com/assetsadapterstore/cxcblock-adapter/cxcblock_addrdec"
	"testing"
)

func TestAddressDecoder_AddressEncode(t *testing.T) {
	cxcblock_addrdec.Default.IsTestNet = false

	p2pk, _ := hex.DecodeString("ba6adceff06b3df8c22dfd00cd11ece4b85ab6f4")
	p2pkAddr, _ := cxcblock_addrdec.Default.AddressEncode(p2pk)
	t.Logf("p2pkAddr: %s", p2pkAddr)

	p2sh, _ := hex.DecodeString("131a861f0609944596e2d618e41ba8ce07b281d0")
	p2shAddr, _ := cxcblock_addrdec.Default.AddressEncode(p2sh, cxcblock_addrdec.CXC_mainnetAddressP2SH)
	t.Logf("p2shAddr: %s", p2shAddr)
}

func TestAddressDecoder_AddressDecode(t *testing.T) {

	cxcblock_addrdec.Default.IsTestNet = false

	p2pkAddr := "VcbvpFHqiwqbsYvaeP71Jw3a11gbCiVyPyU"
	p2pkHash, _ := cxcblock_addrdec.Default.AddressDecode(p2pkAddr)
	t.Logf("p2pkHash: %s", hex.EncodeToString(p2pkHash))

	//p2shAddr := "sQMG5PncvvxVMrVwXpFfBoi3JFHvPiA9aw"
	//
	//p2shHash, _ := cxcblock_addrdec.Default.AddressDecode(p2shAddr, cxcblock_addrdec.CXC_mainnetAddressP2SH)
	//t.Logf("p2shHash: %s", hex.EncodeToString(p2shHash))
}
