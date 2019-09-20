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
	"bufio"
	"fmt"
	"github.com/astaxie/beego/config"
	"github.com/blocktree/openwallet/log"
	"github.com/shopspring/decimal"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var (
	tw *WalletManager
)

func init() {

	tw = testNewWalletManager()
}

func testNewWalletManager() *WalletManager {
	wm := NewWalletManager()

	//读取配置
	absFile := filepath.Join("conf", "conf.ini")
	//log.Debug("absFile:", absFile)
	c, err := config.NewConfig("ini", absFile)
	if err != nil {
		return nil
	}
	wm.LoadAssetsConfig(c)
	//wm.ExplorerClient.Debug = false
	wm.WalletClient.Debug = true
	return wm
}

func TestWalletManager(t *testing.T) {

	t.Log("Symbol:", tw.Config.Symbol)
	t.Log("ServerAPI:", tw.Config.ServerAPI)
}

func TestListUnspent(t *testing.T) {
	utxos, err := tw.ListUnspent(0, "1JoTNMcY7CxzNBsTmYLDy7tjNEqrLCmGGk")
	if err != nil {
		t.Errorf("ListUnspent failed unexpected error: %v\n", err)
		return
	}
	totalBalance := decimal.Zero
	for _, u := range utxos {
		t.Logf("ListUnspent %s: %s = %s\n", u.Address, u.AccountID, u.Amount)
		amount, _ := decimal.NewFromString(u.Amount)
		totalBalance = totalBalance.Add(amount)
	}

	t.Logf("totalBalance: %s \n", totalBalance.String())
}

func TestGetListUnspentByCore(t *testing.T) {
	utxos, err := tw.getListUnspentByCore(0)
	if err != nil {
		t.Errorf("ListUnspent failed unexpected error: %v\n", err)
		return
	}
	totalBalance := decimal.Zero
	for i, u := range utxos {
		t.Logf("ListUnspent[%d] %s: %s = %s\n", i, u.Address, u.AccountID, u.Amount)
		amount, _ := decimal.NewFromString(u.Amount)
		totalBalance = totalBalance.Add(amount)
	}

	t.Logf("totalBalance: %s \n", totalBalance.String())
}


func TestEstimateFee(t *testing.T) {
	feeRate, _ := tw.EstimateFeeRate()
	//feeRate, _ := decimal.NewFromString("0.0001")
	t.Logf("EstimateFee feeRate = %s\n", feeRate.StringFixed(6))
	fees, _ := tw.EstimateFee(10, 1, feeRate)
	t.Logf("EstimateFee fees = %s\n", fees.StringFixed(6))
}

func TestWalletManager_ImportAddress(t *testing.T) {
	addr := "1MasZiznUuxPNYCNGmSoesp1TqoNJCi4tC"
	err := tw.ImportAddress(addr, "")
	if err != nil {
		t.Errorf("RestoreWallet failed unexpected error: %v\n", err)
		return
	}
	log.Info("imported success")
}

func TestWalletManager_Shownet(t *testing.T) {
	tw.Shownet()
}

func TestWalletManager_SendRawTransaction(t *testing.T) {
	_, err := tw.SendRawTransaction("sdfsf")
	if err != nil {
		t.Errorf("SendRawTransaction failed unexpected error: %v\n", err)
		return
	}
}

func TestWalletManager_GetTxOut(t *testing.T) {
	vout, err := tw.GetTxOut("0da1400d6c90d5e0f37886f7b237a64730c39674131c5cb78b25375b523ad64c", 1)
	if err != nil {
		t.Errorf("GetTxOut failed unexpected error: %v\n", err)
		return
	}
	t.Logf("vout: %+v \n", vout)
}

func TestWalletManager_Showaddrs(t *testing.T) {
	addrs, err := tw.Showaddrs()
	if err != nil {
		t.Errorf("Showaddrs failed unexpected error: %v\n", err)
		return
	}
	for i, a := range addrs {
		fmt.Printf("[%d] %+v\n", i, a)
	}
}

func TestWalletManager_Showchain(t *testing.T) {
	tw.Showchain()
}

func TestWalletManager_Addnewaddr(t *testing.T) {
	tw.Addnewaddr()
}

func TestBatchImport(t *testing.T) {
	addrFile := filepath.Join("data", "cxc_addr2.txt")
	addrs, err := readLine(addrFile)
	if err != nil {
		t.Errorf("readLine failed unexpected error: %v\n", err)
		return
	}
	for i, a := range addrs {
		fmt.Printf("[%d] %s\n", i, a)

		err := tw.ImportAddress(a, "")
		if err != nil {
			t.Errorf("ImportAddress failed [%d] unexpected error: %v\n", i, err)
			return
		}
	}
}

func readLine(fileName string) ([]string,error){
	f, err := os.Open(fileName)
	if err != nil {
		return nil,err
	}
	buf := bufio.NewReader(f)
	var result []string
	for {
		line, err := buf.ReadString('\n')
		line = strings.TrimSpace(line)
		if err != nil {
			if err == io.EOF {   //读取结束，会报EOF
				return result,nil
			}
			return nil,err
		}
		result = append(result,line)
	}
	return result,nil
}

