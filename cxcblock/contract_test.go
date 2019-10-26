/*
 * Copyright 2019 The openwallet Authors
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
	"github.com/blocktree/openwallet/log"
	"github.com/blocktree/openwallet/openwallet"
	"testing"
)

func TestWalletManager_ShowAssets(t *testing.T) {
	assets, err := tw.ShowAssets("372121-1930-55680")
	if err != nil {
		t.Errorf("ShowAssets failed unexpected error: %v\n", err)
		return
	}
	log.Infof("assets: %+v", assets)
	sellidhalf := assets.Selltxid[:len(assets.Selltxid)/2]
	log.Infof("sellidhalf: %+v", sellidhalf)
}

func TestWalletManager_GetAssetsBalance(t *testing.T) {
	assets, err := tw.GetAssetsBalance("372121-1930-55680", "1MasZiznUuxPNYCNGmSoesp1TqoNJCi4tC")
	if err != nil {
		t.Errorf("ShowAssets failed unexpected error: %v\n", err)
		return
	}
	log.Infof("assets: %+v", assets)
}

func TestContractDecoder_GetTokenBalanceByAddress(t *testing.T) {
	addr := "1KrKAZyR34ZvK5HRYUHFb6N1WZrWhJd9E5"
	contract := openwallet.SmartContract{
		Address:  "372121-1930-55680",
		Symbol:   "CXC",
		Name:     "TEP",
		Token:    "TEP",
		Decimals: 6,
	}

	balances, err := tw.ContractDecoder.GetTokenBalanceByAddress(contract, addr)
	if err != nil {
		log.Errorf(err.Error())
		return
	}
	for _, b := range balances {
		log.Infof("balance[%s] = %s", b.Balance.Address, b.Balance.Balance)
		log.Infof("UnconfirmBalance[%s] = %s", b.Balance.Address, b.Balance.UnconfirmBalance)
		log.Infof("ConfirmBalance[%s] = %s", b.Balance.Address, b.Balance.ConfirmBalance)
	}
}
