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
	"encoding/json"
	"github.com/blocktree/openwallet/hdkeystore"
	"github.com/blocktree/openwallet/log"
	"github.com/blocktree/openwallet/openwallet"
	"github.com/shopspring/decimal"
	"github.com/tidwall/gjson"
	"math"
)

type WalletManager struct {
	openwallet.AssetsAdapterBase

	Storage      *hdkeystore.HDKeystore        //秘钥存取
	WalletClient *Client                       // 节点客户端
	Config       *WalletConfig                 //钱包管理配置
	Decoder      openwallet.AddressDecoder     //地址编码器
	TxDecoder    openwallet.TransactionDecoder //交易单编码器
	Log          *log.OWLogger                 //日志工具
	Blockscanner *CXCBlockScanner              //区块扫描器
	//ContractDecoder *ContractDecoder              //智能合约解析器
}

func NewWalletManager() *WalletManager {
	wm := WalletManager{}
	wm.Config = NewConfig(Symbol, CurveType, Decimals)
	wm.Decoder = NewAddressDecoder(&wm)
	wm.Log = log.NewOWLogger(wm.Symbol())
	wm.Blockscanner = NewCXCBlockScanner(&wm)
	wm.TxDecoder = NewTransactionDecoder(&wm)
	return &wm
}

func (wm *WalletManager) Shownet() (*gjson.Result, error) {

	result, err := wm.WalletClient.Call("shownet", nil)
	if err != nil {
		return nil, err
	}

	return result, nil
}

//ListUnspent 获取未花记录
func (wm *WalletManager) ListUnspent(min uint64, addresses ...string) ([]*Unspent, error) {

	//:分页限制

	var (
		limit       = 100
		searchAddrs = make([]string, 0)
		max         = len(addresses)
		step        = max / limit
		utxo        = make([]*Unspent, 0)
		pice        []*Unspent
		err         error
	)

	for i := 0; i <= step; i++ {
		begin := i * limit
		end := (i + 1) * limit
		if end > max {
			end = max
		}

		searchAddrs = addresses[begin:end]

		if len(searchAddrs) == 0 {
			continue
		}

		if wm.Config.RPCServerType == RPCServerExplorer {
			//pice, err = wm.listUnspentByExplorer(searchAddrs...)
			//if err != nil {
			//	return nil, err
			//}
			return nil, nil
		} else {
			pice, err = wm.getListUnspentByCore(min, searchAddrs...)
			if err != nil {
				return nil, err
			}
		}
		utxo = append(utxo, pice...)
	}
	return utxo, nil
}

//getTransactionByCore 获取交易单
func (wm *WalletManager) getListUnspentByCore(min uint64, addresses ...string) ([]*Unspent, error) {

	var (
		utxos = make([]*Unspent, 0)
	)

	request := []interface{}{
		min,
		9999999,
	}

	if len(addresses) > 0 {
		request = append(request, addresses)
	}

	result, err := wm.WalletClient.Call("showunspent", request)
	if err != nil {
		return nil, err
	}

	array := result.Array()
	for _, a := range array {
		utxos = append(utxos, NewUnspent(&a))
	}

	return utxos, nil
}

//SendRawTransaction 广播交易
func (wm *WalletManager) SendRawTransaction(txHex string) (string, error) {

	if wm.Config.RPCServerType == RPCServerExplorer {
		//return wm.sendRawTransactionByExplorer(txHex)
		return "", nil
	} else {
		return wm.sendRawTransactionByCore(txHex)
	}
}

//sendRawTransactionByCore 广播交易
func (wm *WalletManager) sendRawTransactionByCore(txHex string) (string, error) {

	request := []interface{}{
		txHex,
	}

	result, err := wm.WalletClient.Call("sendrawdeal", request)
	if err != nil {
		return "", err
	}

	return result.String(), nil

}

//ImportAddress 导入地址核心钱包
func (wm *WalletManager) ImportAddress(address, account string) error {

	request := []interface{}{
		address,
		false,
	}

	_, err := wm.WalletClient.Call("importaddr", request)

	if err != nil {
		return err
	}

	return nil

}

//EstimateFee 预估手续费
func (wm *WalletManager) EstimateFee(inputs, outputs int64, feeRate decimal.Decimal) (decimal.Decimal, error) {

	var piece int64 = 1

	//UTXO如果大于设定限制，则分拆成多笔交易单发送
	if inputs > int64(wm.Config.MaxTxInputs) {
		piece = int64(math.Ceil(float64(inputs) / float64(wm.Config.MaxTxInputs)))
	}

	//计算公式如下：180 * 输入数额 + 34 * 输出数额 + 10
	trx_bytes := decimal.New(inputs*180+outputs*34+piece*10, 0)
	trx_fee := trx_bytes.Div(decimal.New(1000, 0)).Mul(feeRate)
	trx_fee = trx_fee.Round(wm.Decimal())
	//wm.Log.Debugf("trx_fee: %s", trx_fee.String())
	//wm.Log.Debugf("MinFees: %s", wm.Config.MinFees.String())
	//是否低于最小手续费
	if trx_fee.LessThan(wm.Config.MinFees) {
		trx_fee = wm.Config.MinFees
	}

	return trx_fee, nil

	//return wm.Config.MinFees, nil
}

//EstimateFeeRate 预估的没KB手续费率
func (wm *WalletManager) EstimateFeeRate() (decimal.Decimal, error) {

	//if wm.Config.RPCServerType == RPCServerExplorer {
	//	//return wm.estimateFeeRateByExplorer()
	//	return decimal.Zero, nil
	//} else {
	//	return wm.estimateFeeRateByCore()
	//}

	return wm.Config.MinFees, nil
}

//estimateFeeRateByCore 预估的没KB手续费率
func (wm *WalletManager) estimateFeeRateByCore() (decimal.Decimal, error) {

	feeRate := decimal.Zero

	//估算交易大小 手续费
	request := []interface{}{
		2,
	}

	estimatesmartfee, err := wm.WalletClient.Call("estimatesmartfee", request)
	if err != nil {

		estimatefee, err2 := wm.WalletClient.Call("estimatefee", request)
		if err2 != nil {
			return decimal.New(0, 0), err2
		}
		feeRate, _ = decimal.NewFromString(estimatefee.String())
	} else {
		feeRate, _ = decimal.NewFromString(estimatesmartfee.Get("feerate").String())
	}

	return feeRate, nil
}

func (wm *WalletManager) Showaddrs() ([]*Address, error) {

	request := []interface{}{
		"*",
	}

	result, err := wm.WalletClient.Call("showaddrs", request)
	if err != nil {
		return nil, err
	}

	var addrs []*Address
	err = json.Unmarshal([]byte(result.Raw), &addrs)
	if err != nil {
		return nil, err
	}

	return addrs, nil
}

func (wm *WalletManager) Showchain() (*gjson.Result, error) {

	result, err := wm.WalletClient.Call("showchain", nil)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (wm *WalletManager) Addnewaddr() (*gjson.Result, error) {

	result, err := wm.WalletClient.Call("addnewaddr", nil)
	if err != nil {
		return nil, err
	}

	return result, nil
}