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
	"fmt"
	"github.com/blocktree/openwallet/openwallet"
	"github.com/shopspring/decimal"
)


//GetOmniProperty 获取Omni资产信息
func (wm *WalletManager)ShowAssets(propertyId string) (*Assets, error) {

	request := []interface{}{
		propertyId,
	}

	result, err := wm.WalletClient.Call("showassets", request)
	if err != nil {
		return nil, err
	}

	if result.IsArray() {
		assets := result.Array()
		if len(assets) > 0 {
			return NewAssets(assets[0]), nil
		}
	}

	return nil, fmt.Errorf("can not find assets")
}

func (wm *WalletManager) GetAssetsBalance(propertyId, address string) (decimal.Decimal, error) {
	request := []interface{}{
		address,
		propertyId,
		0,
		true,
	}

	result, err := wm.WalletClient.Call("showallbals", request)
	if err != nil {
		return decimal.Zero, err
	}

	assets := result.Get(address)
	if assets.IsArray() {
		for _, asset := range assets.Array() {
			if asset.Get("assetref").String() == propertyId {
				balance, err := decimal.NewFromString(asset.Get("qty").String())
				if err != nil {
					return decimal.Zero, err
				}
				return balance, nil
			}
		}
	}

	return decimal.Zero, nil
}


type ContractDecoder struct {
	*openwallet.SmartContractDecoderBase
	wm *WalletManager
}

//NewContractDecoder 智能合约解析器
func NewContractDecoder(wm *WalletManager) *ContractDecoder {
	decoder := ContractDecoder{}
	decoder.wm = wm
	return &decoder
}

func (decoder *ContractDecoder) GetTokenBalanceByAddress(contract openwallet.SmartContract, address ...string) ([]*openwallet.TokenBalance, error) {

	var tokenBalanceList []*openwallet.TokenBalance

	assetsInfo, err := decoder.wm.ShowAssets(contract.Address)
	if err != nil {
		return nil, err
	}

	for i:=0; i<len(address); i++ {

		balance, err := decoder.wm.GetAssetsBalance(contract.Address, address[i])
		balance = balance.Shift(assetsInfo.Decimals).Shift(-int32(contract.Decimals))
		if err != nil {
			decoder.wm.Log.Errorf("get address[%v] omni token balance failed, err: %v", address[i], err)
		}

		tokenBalance := &openwallet.TokenBalance{
			Contract: &contract,
			Balance: &openwallet.Balance{
				Address:          address[i],
				Symbol:           contract.Symbol,
				Balance:          balance.String(),
				ConfirmBalance:   balance.String(),
				UnconfirmBalance: "0",
			},
		}

		tokenBalanceList = append(tokenBalanceList, tokenBalance)
	}

	return tokenBalanceList, nil
}