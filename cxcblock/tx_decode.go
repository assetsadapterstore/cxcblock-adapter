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
	"errors"
	"fmt"
	"github.com/blocktree/go-owcdrivers/cxcTransaction"
	"github.com/blocktree/openwallet/openwallet"
	"github.com/shopspring/decimal"
	"sort"
	"strings"
	"time"
)

type TransactionDecoder struct {
	openwallet.TransactionDecoderBase
	wm *WalletManager //钱包管理者
}

//NewTransactionDecoder 交易单解析器
func NewTransactionDecoder(wm *WalletManager) *TransactionDecoder {
	decoder := TransactionDecoder{}
	decoder.wm = wm
	return &decoder
}

//CreateRawTransaction 创建交易单
func (decoder *TransactionDecoder) CreateRawTransaction(wrapper openwallet.WalletDAI, rawTx *openwallet.RawTransaction) error {
	if rawTx.Coin.IsContract {
		return decoder.CreateTokenRawTransaction(wrapper, rawTx)
	} else {
		return decoder.CreateCXCRawTransaction(wrapper, rawTx)
	}
}

//SignRawTransaction 签名交易单
func (decoder *TransactionDecoder) SignRawTransaction(wrapper openwallet.WalletDAI, rawTx *openwallet.RawTransaction) error {
	return decoder.SignCXCRawTransaction(wrapper, rawTx)
}

//VerifyRawTransaction 验证交易单，验证交易单并返回加入签名后的交易单
func (decoder *TransactionDecoder) VerifyRawTransaction(wrapper openwallet.WalletDAI, rawTx *openwallet.RawTransaction) error {
	return decoder.VerifyCXCRawTransaction(wrapper, rawTx)
}

//CreateSummaryRawTransaction 创建汇总交易，返回原始交易单数组
func (decoder *TransactionDecoder) CreateSummaryRawTransaction(wrapper openwallet.WalletDAI, sumRawTx *openwallet.SummaryRawTransaction) ([]*openwallet.RawTransaction, error) {
	var (
		rawTxWithErrArray []*openwallet.RawTransactionWithError
		rawTxArray        = make([]*openwallet.RawTransaction, 0)
		err               error
	)
	if sumRawTx.Coin.IsContract {
		//return nil, fmt.Errorf("contract transfer is not support")
		rawTxWithErrArray, err = decoder.CreateTokenSummaryRawTransaction(wrapper, sumRawTx)
	} else {
		rawTxWithErrArray, err = decoder.CreateBTCSummaryRawTransaction(wrapper, sumRawTx)
	}
	if err != nil {
		return nil, err
	}
	for _, rawTxWithErr := range rawTxWithErrArray {
		if rawTxWithErr.Error != nil {
			continue
		}
		rawTxArray = append(rawTxArray, rawTxWithErr.RawTx)
	}
	return rawTxArray, nil
}

//SendRawTransaction 广播交易单
func (decoder *TransactionDecoder) SubmitRawTransaction(wrapper openwallet.WalletDAI, rawTx *openwallet.RawTransaction) (*openwallet.Transaction, error) {

	if len(rawTx.RawHex) == 0 {
		return nil, fmt.Errorf("transaction hex is empty")
	}

	if !rawTx.IsCompleted {
		return nil, fmt.Errorf("transaction is not completed validation")
	}

	txid, err := decoder.wm.SendRawTransaction(rawTx.RawHex)
	if err != nil {
		decoder.wm.Log.Warningf("[Sid: %s] submit raw hex: %s", rawTx.Sid, rawTx.RawHex)
		return nil, err
	}

	rawTx.TxID = txid
	rawTx.IsSubmit = true

	decimals := int32(0)
	fees := "0"
	if rawTx.Coin.IsContract {
		decimals = int32(rawTx.Coin.Contract.Decimals)
		fees = "0"
	} else {
		decimals = int32(decoder.wm.Decimal())
		fees = rawTx.Fees
	}

	//记录一个交易单
	tx := &openwallet.Transaction{
		From:       rawTx.TxFrom,
		To:         rawTx.TxTo,
		Amount:     rawTx.TxAmount,
		Coin:       rawTx.Coin,
		TxID:       rawTx.TxID,
		Decimal:    decimals,
		AccountID:  rawTx.Account.AccountID,
		Fees:       fees,
		SubmitTime: time.Now().Unix(),
	}

	tx.WxID = openwallet.GenTransactionWxID(tx)

	return tx, nil
}

////////////////////////// BTC implement //////////////////////////

//CreateRawTransaction 创建交易单
func (decoder *TransactionDecoder) CreateCXCRawTransaction(wrapper openwallet.WalletDAI, rawTx *openwallet.RawTransaction) error {

	var (
		usedUTXO     []*Unspent
		outputAddrs  = make(map[string]decimal.Decimal)
		balance      = decimal.New(0, 0)
		totalSend    = decimal.New(0, 0)
		actualFees   = decimal.New(0, 0)
		feesRate     = decimal.New(0, 0)
		accountID    = rawTx.Account.AccountID
		destinations = make([]string, 0)
		//accountTotalSent = decimal.Zero
		limit = 2000
	)

	address, err := wrapper.GetAddressList(0, limit, "AccountID", rawTx.Account.AccountID)
	if err != nil {
		return err
	}

	if len(address) == 0 {
		return openwallet.Errorf(openwallet.ErrAccountNotAddress, "[%s] have not addresses", accountID)
		//return fmt.Errorf("[%s] have not addresses", accountID)
	}

	searchAddrs := make([]string, 0)
	for _, address := range address {
		searchAddrs = append(searchAddrs, address.Address)
	}
	//decoder.wm.Log.Debug(searchAddrs)
	//查找账户的utxo, 项目方说要6个确认才可用
	unspents, err := decoder.wm.ListUnspent(0, searchAddrs...)
	if err != nil {
		return err
	}

	if len(unspents) == 0 {
		return openwallet.Errorf(openwallet.ErrInsufficientBalanceOfAccount, "[%s] balance is not enough(must over 6 confirmations)", accountID)
	}

	if len(rawTx.To) == 0 {
		return errors.New("Receiver addresses is empty!")
	}

	//计算总发送金额
	for addr, amount := range rawTx.To {
		deamount, _ := decimal.NewFromString(amount)
		totalSend = totalSend.Add(deamount)
		destinations = append(destinations, addr)
		//计算账户的实际转账amount
		//addresses, findErr := wrapper.GetAddressList(0, -1, "AccountID", rawTx.Account.AccountID, "Address", addr)
		//if findErr != nil || len(addresses) == 0 {
		//	amountDec, _ := decimal.NewFromString(amount)
		//	accountTotalSent = accountTotalSent.Add(amountDec)
		//}
	}

	//获取utxo，按小到大排序
	sort.Sort(UnspentSort{unspents, func(a, b *Unspent) int {
		a_amount, _ := decimal.NewFromString(a.Amount)
		b_amount, _ := decimal.NewFromString(b.Amount)
		if a_amount.GreaterThan(b_amount) {
			return 1
		} else {
			return -1
		}
	}})

	if len(rawTx.FeeRate) == 0 {
		feesRate, err = decoder.wm.EstimateFeeRate()
		if err != nil {
			return err
		}
	} else {
		feesRate, _ = decimal.NewFromString(rawTx.FeeRate)
	}

	decoder.wm.Log.Info("Calculating wallet unspent record to build transaction...")
	computeTotalSend := totalSend
	//循环的计算余额是否足够支付发送数额+手续费
	for {

		usedUTXO = make([]*Unspent, 0)
		balance = decimal.New(0, 0)

		//计算一个可用于支付的余额
		for _, u := range unspents {

			if u.Spendable {
				ua, _ := decimal.NewFromString(u.Amount)
				if ua.GreaterThan(decimal.Zero) {
					balance = balance.Add(ua)
					usedUTXO = append(usedUTXO, u)
					if balance.GreaterThanOrEqual(computeTotalSend) {
						break
					}
				}
			}
		}

		if balance.LessThan(computeTotalSend) {
			return openwallet.Errorf(openwallet.ErrInsufficientBalanceOfAccount, "The balance: %s is not enough! ", balance.StringFixed(decoder.wm.Decimal()))
		}

		//计算手续费，找零地址有2个，一个是发送，一个是新创建的
		fees, err := decoder.wm.EstimateFee(int64(len(usedUTXO)), int64(len(destinations)+1), feesRate)
		if err != nil {
			return err
		}

		//如果要手续费有发送支付，得计算加入手续费后，计算余额是否足够
		//总共要发送的
		computeTotalSend = totalSend.Add(fees)
		if computeTotalSend.GreaterThan(balance) {
			continue
		}
		computeTotalSend = totalSend

		actualFees = fees

		break

	}

	//UTXO如果大于设定限制，则分拆成多笔交易单发送
	if len(usedUTXO) > decoder.wm.Config.MaxTxInputs {
		errStr := fmt.Sprintf("The transaction is use max inputs over: %d", decoder.wm.Config.MaxTxInputs)
		return errors.New(errStr)
	}

	//取账户最后一个地址
	changeAddress := usedUTXO[0].Address

	changeAmount := balance.Sub(computeTotalSend).Sub(actualFees)
	rawTx.FeeRate = feesRate.StringFixed(decoder.wm.Decimal())
	rawTx.Fees = actualFees.StringFixed(decoder.wm.Decimal())

	decoder.wm.Log.Std.Notice("-----------------------------------------------")
	decoder.wm.Log.Std.Notice("From Account: %s", accountID)
	decoder.wm.Log.Std.Notice("To Address: %s", strings.Join(destinations, ", "))
	decoder.wm.Log.Std.Notice("Use: %v", balance.StringFixed(decoder.wm.Decimal()))
	decoder.wm.Log.Std.Notice("Fees: %v", actualFees.StringFixed(decoder.wm.Decimal()))
	decoder.wm.Log.Std.Notice("Receive: %v", computeTotalSend.StringFixed(decoder.wm.Decimal()))
	decoder.wm.Log.Std.Notice("Change: %v", changeAmount.StringFixed(decoder.wm.Decimal()))
	decoder.wm.Log.Std.Notice("Change Address: %v", changeAddress)
	decoder.wm.Log.Std.Notice("-----------------------------------------------")

	//装配输出
	for to, amount := range rawTx.To {
		decamount, _ := decimal.NewFromString(amount)
		outputAddrs = appendOutput(outputAddrs, to, decamount)
		//outputAddrs[to] = amount
	}

	//changeAmount := balance.Sub(totalSend).Sub(actualFees)
	if changeAmount.GreaterThan(decimal.New(0, 0)) {
		outputAddrs = appendOutput(outputAddrs, changeAddress, changeAmount)
		//outputAddrs[changeAddress] = changeAmount.StringFixed(decoder.wm.Decimal())
	}

	err = decoder.createCXCRawTransaction(wrapper, rawTx, usedUTXO, outputAddrs)
	if err != nil {
		return err
	}

	return nil
}

//SignRawTransaction 签名交易单
func (decoder *TransactionDecoder) SignCXCRawTransaction(wrapper openwallet.WalletDAI, rawTx *openwallet.RawTransaction) error {

	if rawTx.Signatures == nil || len(rawTx.Signatures) == 0 {
		//this.wm.Log.Std.Error("len of signatures error. ")
		return fmt.Errorf("transaction signature is empty")
	}

	key, err := wrapper.HDKey()
	if err != nil {
		return err
	}

	//keySignatures := rawTx.Signatures[rawTx.Account.AccountID]
	for accountID, keySignatures := range rawTx.Signatures {
		decoder.wm.Log.Debug("accountID:", accountID)
		if keySignatures != nil {
			for _, keySignature := range keySignatures {

				childKey, err := key.DerivedKeyWithPath(keySignature.Address.HDPath, keySignature.EccType)
				keyBytes, err := childKey.GetPrivateKeyBytes()
				if err != nil {
					return err
				}
				//decoder.wm.Log.Debug("privateKey:", hex.EncodeToString(keyBytes))

				//privateKeys = append(privateKeys, keyBytes)
				txHash := cxcTransaction.TxHash{
					Hash: keySignature.Message,
					Normal: &cxcTransaction.NormalTx{
						Address: keySignature.Address.Address,
						SigType: cxcTransaction.SigHashAll,
					},
				}
				//transHash = append(transHash, txHash)

				decoder.wm.Log.Debug("hash:", txHash.GetTxHashHex())

				//签名交易
				/////////交易单哈希签名
				sigPub, err := cxcTransaction.SignRawTransactionHash(txHash.GetTxHashHex(), keyBytes)
				if err != nil {
					return fmt.Errorf("transaction hash sign failed, unexpected error: %v", err)
				} else {

					//for i, s := range sigPub {
					//	decoder.wm.Log.Info("第", i+1, "个签名结果")
					//	decoder.wm.Log.Info()
					//	decoder.wm.Log.Info("对应的公钥为")
					//	decoder.wm.Log.Info(hex.EncodeToString(s.Pubkey))
					//}

					//txHash.Normal.SigPub = *sigPub
				}

				keySignature.Signature = hex.EncodeToString(sigPub.Signature)
			}
		}

		rawTx.Signatures[accountID] = keySignatures
	}

	decoder.wm.Log.Info("transaction hash sign success")

	//rawTx.Signatures[rawTx.Account.AccountID] = keySignatures

	//decoder.wm.Log.Info("rawTx.Signatures 1:", rawTx.Signatures)

	return nil
}

//VerifyRawTransaction 验证交易单，验证交易单并返回加入签名后的交易单
func (decoder *TransactionDecoder) VerifyCXCRawTransaction(wrapper openwallet.WalletDAI, rawTx *openwallet.RawTransaction) error {

	//先加载是否有配置文件
	//err := decoder.wm.LoadConfig()
	//if err != nil {
	//	return err
	//}

	var (
		txUnlocks  = make([]cxcTransaction.TxUnlock, 0)
		emptyTrans = rawTx.RawHex
		//sigPub     = make([]cxcTransaction.SignaturePubkey, 0)
		transHash     = make([]cxcTransaction.TxHash, 0)
		addressPrefix cxcTransaction.AddressPrefix
	)

	if rawTx.Signatures == nil || len(rawTx.Signatures) == 0 {
		//this.wm.Log.Std.Error("len of signatures error. ")
		return fmt.Errorf("transaction signature is empty")
	}

	for accountID, keySignatures := range rawTx.Signatures {
		decoder.wm.Log.Debug("accountID Signatures:", accountID)
		for _, keySignature := range keySignatures {

			signature, _ := hex.DecodeString(keySignature.Signature)
			pubkey, _ := hex.DecodeString(keySignature.Address.PublicKey)

			signaturePubkey := cxcTransaction.SignaturePubkey{
				Signature: signature,
				Pubkey:    pubkey,
			}

			//sigPub = append(sigPub, signaturePubkey)

			txHash := cxcTransaction.TxHash{
				Hash: keySignature.Message,
				Normal: &cxcTransaction.NormalTx{
					Address: keySignature.Address.Address,
					SigType: cxcTransaction.SigHashAll,
					SigPub:  signaturePubkey,
				},
			}

			transHash = append(transHash, txHash)

			decoder.wm.Log.Debug("Signature:", keySignature.Signature)
			decoder.wm.Log.Debug("PublicKey:", keySignature.Address.PublicKey)
		}
	}

	txBytes, err := hex.DecodeString(emptyTrans)
	if err != nil {
		return errors.New("Invalid transaction hex data!")
	}

	trx, err := cxcTransaction.DecodeRawTransaction(txBytes, decoder.wm.Config.SupportSegWit)
	if err != nil {
		return errors.New("Invalid transaction data! ")
	}

	for i, vin := range trx.Vins {

		utxo, err := decoder.wm.GetTxOut(vin.GetTxID(), uint64(vin.GetVout()))
		if err != nil {
			return err
		}

		txUnlock := cxcTransaction.TxUnlock{
			LockScript: utxo.ScriptPubKey,
			SigType:    cxcTransaction.SigHashAll}
		txUnlocks = append(txUnlocks, txUnlock)

		transHash = resetTransHashFunc(transHash, utxo.Addr, i)
	}

	//decoder.wm.Log.Debug(emptyTrans)

	////////填充签名结果到空交易单
	//  传入TxUnlock结构体的原因是： 解锁向脚本支付的UTXO时需要对应地址的赎回脚本， 当前案例的对应字段置为 "" 即可
	signedTrans, err := cxcTransaction.InsertSignatureIntoEmptyTransaction(emptyTrans, transHash, txUnlocks, decoder.wm.Config.SupportSegWit)
	if err != nil {
		return fmt.Errorf("transaction compose signatures failed")
	}
	//else {
	//	//	fmt.Println("拼接后的交易单")
	//	//	fmt.Println(signedTrans)
	//	//}

	if decoder.wm.Config.IsTestNet {
		addressPrefix = decoder.wm.Config.TestNetAddressPrefix
	} else {
		addressPrefix = decoder.wm.Config.MainNetAddressPrefix
	}

	/////////验证交易单
	//验证时，对于公钥哈希地址，需要将对应的锁定脚本传入TxUnlock结构体
	pass := cxcTransaction.VerifyRawTransaction(signedTrans, txUnlocks, decoder.wm.Config.SupportSegWit, addressPrefix)
	if pass {
		decoder.wm.Log.Debug("transaction verify passed")
		rawTx.IsCompleted = true
		rawTx.RawHex = signedTrans
	} else {
		decoder.wm.Log.Debug("transaction verify failed")
		rawTx.IsCompleted = false
	}

	return nil
}

//GetRawTransactionFeeRate 获取交易单的费率
func (decoder *TransactionDecoder) GetRawTransactionFeeRate() (feeRate string, unit string, err error) {
	rate, err := decoder.wm.EstimateFeeRate()
	if err != nil {
		return "", "", err
	}

	return rate.StringFixed(decoder.wm.Decimal()), "TX", nil
}

//CreateBTCSummaryRawTransaction 创建BTC汇总交易
func (decoder *TransactionDecoder) CreateBTCSummaryRawTransaction(wrapper openwallet.WalletDAI, sumRawTx *openwallet.SummaryRawTransaction) ([]*openwallet.RawTransactionWithError, error) {

	var (
		feesRate       = decimal.New(0, 0)
		accountID      = sumRawTx.Account.AccountID
		minTransfer, _ = decimal.NewFromString(sumRawTx.MinTransfer)
		//retainedBalance, _ = decimal.NewFromString(sumRawTx.RetainedBalance)
		sumAddresses     = make([]string, 0)
		rawTxArray       = make([]*openwallet.RawTransactionWithError, 0)
		sumUnspents      []*Unspent
		outputAddrs      map[string]decimal.Decimal
		totalInputAmount decimal.Decimal
	)

	//if minTransfer.LessThan(retainedBalance) {
	//	return nil, fmt.Errorf("mini transfer amount must be greater than address retained balance")
	//}

	address, err := wrapper.GetAddressList(sumRawTx.AddressStartIndex, sumRawTx.AddressLimit, "AccountID", sumRawTx.Account.AccountID)
	if err != nil {
		return nil, err
	}

	if len(address) == 0 {
		return nil, fmt.Errorf("[%s] have not addresses", accountID)
	}

	searchAddrs := make([]string, 0)
	for _, address := range address {
		searchAddrs = append(searchAddrs, address.Address)
	}

	addrBalanceArray, err := decoder.wm.Blockscanner.GetBalanceByAddress(searchAddrs...)
	if err != nil {
		return nil, err
	}

	for _, addrBalance := range addrBalanceArray {
		decoder.wm.Log.Debugf("addrBalance: %+v", addrBalance)
		//检查余额是否超过最低转账
		addrBalance_dec, _ := decimal.NewFromString(addrBalance.Balance)
		if addrBalance_dec.GreaterThanOrEqual(minTransfer) {
			//添加到转账地址数组
			sumAddresses = append(sumAddresses, addrBalance.Address)
		}
	}

	if len(sumAddresses) == 0 {
		return nil, nil
	}

	//取得费率
	if len(sumRawTx.FeeRate) == 0 {
		feesRate, err = decoder.wm.EstimateFeeRate()
		if err != nil {
			return nil, err
		}
	} else {
		feesRate, _ = decimal.NewFromString(sumRawTx.FeeRate)
	}

	sumUnspents = make([]*Unspent, 0)
	outputAddrs = make(map[string]decimal.Decimal, 0)
	totalInputAmount = decimal.Zero

	for i, addr := range sumAddresses {

		unspents, err := decoder.wm.ListUnspent(sumRawTx.Confirms, addr)
		if err != nil {
			return nil, err
		}

		//保留1个token的最低转账成本的utxo 用于汇总token
		//unspents = decoder.keepTokenCostUTXONotToUse(unspents)

		//尽可能筹够最大input数
		if len(unspents)+len(sumUnspents) < decoder.wm.Config.MaxTxInputs {
			sumUnspents = append(sumUnspents, unspents...)
			//if retainedBalance.GreaterThan(decimal.Zero) {
			//	outputAddrs = appendOutput(outputAddrs, addr, retainedBalance)
			//outputAddrs[addr] = retainedBalance.StringFixed(decoder.wm.Decimal())
			//}
			//decoder.wm.Log.Debugf("sumUnspents: %+v", sumUnspents)
		}

		//如果utxo已经超过最大输入，或遍历地址完结，就可以进行构建交易单
		if i == len(sumAddresses)-1 || len(sumUnspents) >= decoder.wm.Config.MaxTxInputs {
			//执行构建交易单工作
			//decoder.wm.Log.Debugf("sumUnspents: %+v", sumUnspents)
			//计算手续费，构建交易单inputs，地址保留余额>0，地址需要加入输出，最后+1是汇总地址
			fees, createErr := decoder.wm.EstimateFee(int64(len(sumUnspents)), int64(len(outputAddrs)+1), feesRate)
			if createErr != nil {
				return nil, createErr
			}

			//计算这笔交易单的汇总数量
			for _, u := range sumUnspents {

				if u.Spendable {
					ua, _ := decimal.NewFromString(u.Amount)
					totalInputAmount = totalInputAmount.Add(ua)
				}
			}

			/*

					汇总数量计算：

					1. 输入总数量 = 合计账户地址的所有utxo
					2. 账户地址输出总数量 = 账户地址保留余额 * 地址数
				    3. 汇总数量 = 输入总数量 - 账户地址输出总数量 - 手续费
			*/
			//retainedBalanceTotal := retainedBalance.Mul(decimal.New(int64(len(outputAddrs)), 0))
			sumAmount := totalInputAmount.Sub(fees)

			decoder.wm.Log.Debugf("totalInputAmount: %v", totalInputAmount)
			//decoder.wm.Log.Debugf("retainedBalanceTotal: %v", retainedBalanceTotal)
			decoder.wm.Log.Debugf("fees: %v", fees)
			decoder.wm.Log.Debugf("sumAmount: %v", sumAmount)

			if sumAmount.GreaterThan(decimal.Zero) {

				//最后填充汇总地址及汇总数量
				outputAddrs = appendOutput(outputAddrs, sumRawTx.SummaryAddress, sumAmount)
				//outputAddrs[sumRawTx.SummaryAddress] = sumAmount.StringFixed(decoder.wm.Decimal())

				raxTxTo := make(map[string]string, 0)
				for a, m := range outputAddrs {
					raxTxTo[a] = m.StringFixed(decoder.wm.Decimal())
				}

				//创建一笔交易单
				rawTx := &openwallet.RawTransaction{
					Coin:     sumRawTx.Coin,
					Account:  sumRawTx.Account,
					FeeRate:  sumRawTx.FeeRate,
					To:       raxTxTo,
					Fees:     fees.StringFixed(decoder.wm.Decimal()),
					Required: 1,
				}

				usedUTXO := make([]*Unspent, 0)
				for _, u := range sumUnspents {
					ua, _ := decimal.NewFromString(u.Amount)
					if ua.GreaterThan(decimal.Zero) {
						usedUTXO = append(usedUTXO, u)
					}
				}

				createErr = decoder.createCXCRawTransaction(wrapper, rawTx, usedUTXO, outputAddrs)
				rawTxWithErr := &openwallet.RawTransactionWithError{
					RawTx: rawTx,
					Error: openwallet.ConvertError(createErr),
				}

				//创建成功，添加到队列
				rawTxArray = append(rawTxArray, rawTxWithErr)

			}

			//清空临时变量
			sumUnspents = make([]*Unspent, 0)
			outputAddrs = make(map[string]decimal.Decimal, 0)
			totalInputAmount = decimal.Zero

		}
	}

	return rawTxArray, nil
}

//createCXCRawTransaction 创建BTC原始交易单
func (decoder *TransactionDecoder) createCXCRawTransaction(
	wrapper openwallet.WalletDAI,
	rawTx *openwallet.RawTransaction,
	usedUTXO []*Unspent,
	to map[string]decimal.Decimal,
) error {

	var (
		err              error
		vins             = make([]cxcTransaction.Vin, 0)
		vouts            = make([]cxcTransaction.Vout, 0)
		txUnlocks        = make([]cxcTransaction.TxUnlock, 0)
		totalSend        = decimal.New(0, 0)
		destinations     = make([]string, 0)
		accountTotalSent = decimal.Zero
		txFrom           = make([]string, 0)
		txTo             = make([]string, 0)
		accountID        = rawTx.Account.AccountID
		addressPrefix    cxcTransaction.AddressPrefix
	)

	if len(usedUTXO) == 0 {
		return fmt.Errorf("utxo is empty")
	}

	if len(to) == 0 {
		return fmt.Errorf("Receiver addresses is empty! ")
	}

	//计算总发送金额
	for addr, amount := range to {
		//deamount, _ := decimal.NewFromString(amount)
		totalSend = totalSend.Add(amount)
		destinations = append(destinations, addr)
		//计算账户的实际转账amount
		addresses, findErr := wrapper.GetAddressList(0, -1, "AccountID", accountID, "Address", addr)
		if findErr != nil || len(addresses) == 0 {
			//amountDec, _ := decimal.NewFromString(amount)
			accountTotalSent = accountTotalSent.Add(amount)
		}
	}

	//UTXO如果大于设定限制，则分拆成多笔交易单发送
	if len(usedUTXO) > decoder.wm.Config.MaxTxInputs {
		errStr := fmt.Sprintf("The transaction is use max inputs over: %d", decoder.wm.Config.MaxTxInputs)
		return errors.New(errStr)
	}

	//装配输入
	for _, utxo := range usedUTXO {
		in := cxcTransaction.Vin{utxo.TxID, uint32(utxo.Vout)}
		vins = append(vins, in)

		txUnlock := cxcTransaction.TxUnlock{LockScript: utxo.ScriptPubKey, SigType: cxcTransaction.SigHashAll}
		txUnlocks = append(txUnlocks, txUnlock)

		txFrom = append(txFrom, fmt.Sprintf("%s:%s", utxo.Address, utxo.Amount))
	}

	//装配输入
	for to, amount := range to {
		txTo = append(txTo, fmt.Sprintf("%s:%s", to, amount.String()))
		amount = amount.Shift(decoder.wm.Decimal())
		out := cxcTransaction.Vout{to, uint64(amount.IntPart())}
		vouts = append(vouts, out)
	}

	//锁定时间
	lockTime := uint32(0)

	//追加手续费支持
	replaceable := false

	if decoder.wm.Config.IsTestNet {
		addressPrefix = decoder.wm.Config.TestNetAddressPrefix
	} else {
		addressPrefix = decoder.wm.Config.MainNetAddressPrefix
	}

	/////////构建空交易单
	emptyTrans, err := cxcTransaction.CreateEmptyRawTransaction(vins, vouts, lockTime, replaceable, addressPrefix)

	if err != nil {
		return fmt.Errorf("create transaction failed, unexpected error: %v", err)
		//decoder.wm.Log.Error("构建空交易单失败")
	}

	////////构建用于签名的交易单哈希
	transHash, err := cxcTransaction.CreateRawTransactionHashForSig(emptyTrans, txUnlocks, decoder.wm.Config.SupportSegWit, addressPrefix)
	if err != nil {
		return fmt.Errorf("create transaction hash for sig failed, unexpected error: %v", err)
		//decoder.wm.Log.Error("获取待签名交易单哈希失败")
	}

	rawTx.RawHex = emptyTrans

	if rawTx.Signatures == nil {
		rawTx.Signatures = make(map[string][]*openwallet.KeySignature)
	}

	//装配签名
	keySigs := make([]*openwallet.KeySignature, 0)

	for i, txHash := range transHash {

		var unlockAddr string

		//txHash := transHash[i]

		//判断是否是多重签名
		if txHash.IsMultisig() {
			//获取地址
			//unlockAddr = txHash.GetMultiTxPubkeys() //返回hex数组
		} else {
			//获取地址
			unlockAddr = txHash.GetNormalTxAddress() //返回hex串
		}
		//获取hash值
		beSignHex := txHash.GetTxHashHex()

		decoder.wm.Log.Std.Debug("txHash[%d]: %s", i, beSignHex)
		//beSignHex := transHash[i]

		addr, err := wrapper.GetAddress(unlockAddr)
		if err != nil {
			return err
		}

		signature := openwallet.KeySignature{
			EccType: decoder.wm.Config.CurveType,
			Nonce:   "",
			Address: addr,
			Message: beSignHex,
		}

		keySigs = append(keySigs, &signature)

	}

	feesDec, _ := decimal.NewFromString(rawTx.Fees)
	accountTotalSent = accountTotalSent.Add(feesDec)
	accountTotalSent = decimal.Zero.Sub(accountTotalSent)

	//TODO:多重签名要使用owner的公钥填充

	rawTx.Signatures[rawTx.Account.AccountID] = keySigs
	rawTx.IsBuilt = true
	rawTx.TxAmount = accountTotalSent.StringFixed(decoder.wm.Decimal())
	rawTx.TxFrom = txFrom
	rawTx.TxTo = txTo

	return nil
}

////////////////////////// tokencore implement //////////////////////////

//CreateTokenRawTransaction 创建Token交易单
func (decoder *TransactionDecoder) CreateTokenRawTransaction(wrapper openwallet.WalletDAI, rawTx *openwallet.RawTransaction) error {

	var (
		outputAddrs      = make(map[string]decimal.Decimal)
		tokenOutputAddrs = make(map[string]decimal.Decimal)

		toAddress string
		toAmount  = decimal.Zero

		addressAllUTXO  = make([]*Unspent, 0)
		availableUTXO   = make([]*Unspent, 0)
		usedUTXO        = make([]*Unspent, 0)
		useTokenBalance = decimal.Zero
		balance         = decimal.New(0, 0)
		actualFees      = decimal.New(0, 0)
		feesRate        = decimal.New(0, 0)
		accountID       = rawTx.Account.AccountID

		//accountTotalSent = decimal.Zero
		//txFrom           = make([]string, 0)
		//txTo             = make([]string, 0)
	)

	if len(rawTx.Coin.Contract.Address) == 0 {
		return fmt.Errorf("contract address is empty")
	}

	//Token代币编号
	propertyID := rawTx.Coin.Contract.Address
	tokenCoin := rawTx.Coin.Contract.Token
	tokenDecimals := int32(rawTx.Coin.Contract.Decimals)
	//转账最低成本
	//transferCost, _ := decimal.NewFromString(decoder.wm.Config.TokenTransferCost)

	address, err := wrapper.GetAddressList(0, -1, "AccountID", rawTx.Account.AccountID)
	if err != nil {
		return err
	}

	if len(address) == 0 {
		return fmt.Errorf("[%s] have not addresses", accountID)
	}

	if len(rawTx.To) == 0 {
		return errors.New("Receiver addresses is empty!")
	}

	//token限制只能发送目标一个
	if len(rawTx.To) > 1 {
		return fmt.Errorf("ommni transfer not support multiple receiver address")
	}

	//选择一个输出地址
	for to, amount := range rawTx.To {
		toAddress = to
		toAmount, _ = decimal.NewFromString(amount)

		//计算账户的实际转账amount
		//addresses, findErr := wrapper.GetAddressList(0, -1, "AccountID", rawTx.Account.AccountID, "Address", to)
		//if findErr != nil || len(addresses) == 0 {
		//	accountTotalSent = accountTotalSent.Add(toAmount)
		//}
	}

	for _, address := range address {

		//查找地址token余额
		tokenBalance, checkErr := decoder.wm.GetAssetsBalance(propertyID, address.Address)
		if checkErr != nil {
			continue
		}

		if tokenBalance.LessThan(toAmount) {
			continue
		}

		//查找账户的utxo
		unspents, tokenErr := decoder.wm.ListUnspent(0, address.Address)
		if tokenErr != nil {
			continue
		}

		availableAssetsUTXO, availableBalance, terr := decoder.wm.getAvailableUnspentByAssets(unspents, propertyID, toAmount)
		if terr != nil {
			continue
		}

		useTokenBalance = availableBalance
		availableUTXO = availableAssetsUTXO
		addressAllUTXO = unspents
	}

	//单个地址的可用Token余额不足够
	if useTokenBalance.LessThan(toAmount) {
		return openwallet.Errorf(openwallet.ErrInsufficientBalanceOfAddress, "account[%s] token[%s] total balance: %s is not enough! ", accountID, tokenCoin, useTokenBalance.String())
	}

	//获取手续费率
	if len(rawTx.FeeRate) == 0 {
		feesRate, err = decoder.wm.EstimateFeeRate()
		if err != nil {
			return err
		}
	} else {
		feesRate, _ = decimal.NewFromString(rawTx.FeeRate)
	}

	decoder.wm.Log.Info("Calculating wallet unspent record to build transaction...")
	//computeTotalSend := transferCost
	computeTotalSend := decimal.Zero
	//循环的计算余额是否足够支付发送数额+手续费
	for {

		usedUTXO = availableUTXO
		balance = decimal.Zero

		//计算一个可用于支付的余额
		for _, u := range addressAllUTXO {

			if u.Spendable {
				ua, _ := decimal.NewFromString(u.Amount)
				if ua.GreaterThan(decimal.Zero) {
					balance = balance.Add(ua)
					usedUTXO = append(usedUTXO, u)
					if balance.GreaterThanOrEqual(computeTotalSend) {
						break
					}
				}
			}
		}

		if balance.LessThan(computeTotalSend) {
			return openwallet.Errorf(openwallet.ErrInsufficientFees, "The [%s] available utxo balance: %s is not enough! ", decoder.wm.Symbol(), balance.StringFixed(decoder.wm.Decimal()))
		}

		//计算手续费，输出地址有2个，一个是发送，一个是找零，一个是op_reture
		fees, err := decoder.wm.EstimateFee(int64(len(usedUTXO)), int64(3), feesRate)
		if err != nil {
			return err
		}

		//如果要手续费有发送支付，得计算加入手续费后，计算余额是否足够
		//总共要发送的
		computeTotalSend = fees
		if computeTotalSend.GreaterThan(balance) {
			continue
		}
		computeTotalSend = decimal.Zero

		actualFees = fees

		break

	}

	//UTXO如果大于设定限制，则分拆成多笔交易单发送
	if len(usedUTXO) > decoder.wm.Config.MaxTxInputs {
		errStr := fmt.Sprintf("The transaction is use max inputs over: %d", decoder.wm.Config.MaxTxInputs)
		return errors.New(errStr)
	}

	//取账户最后一个地址
	changeAddress := usedUTXO[0].Address

	changeAmount := balance.Sub(actualFees)
	changeTokenAmount := useTokenBalance.Sub(toAmount)
	rawTx.FeeRate = feesRate.StringFixed(decoder.wm.Decimal())
	rawTx.Fees = actualFees.StringFixed(decoder.wm.Decimal())

	decoder.wm.Log.Std.Notice("-----------------------------------------------")
	decoder.wm.Log.Std.Notice("From Account: %s", accountID)
	decoder.wm.Log.Std.Notice("To Address: %s", toAddress)
	decoder.wm.Log.Std.Notice("Amount %s: %v", tokenCoin, toAmount.StringFixed(tokenDecimals))
	decoder.wm.Log.Std.Notice("Use %s: %v", decoder.wm.Symbol(), balance.StringFixed(decoder.wm.Decimal()))
	decoder.wm.Log.Std.Notice("Fees %s: %v", decoder.wm.Symbol(), actualFees.StringFixed(decoder.wm.Decimal()))
	decoder.wm.Log.Std.Notice("Change %s: %v", decoder.wm.Symbol(), changeAmount.StringFixed(decoder.wm.Decimal()))
	decoder.wm.Log.Std.Notice("Change Token %s: %v", tokenCoin, changeTokenAmount.StringFixed(decoder.wm.Decimal()))
	decoder.wm.Log.Std.Notice("Change Address: %v", changeAddress)
	decoder.wm.Log.Std.Notice("-----------------------------------------------")

	//outputAddrs = appendOutput(outputAddrs, toAddress, computeTotalSend)
	//outputAddrs[toAddress] = computeTotalSend.StringFixed(decoder.wm.Decimal())

	//changeAmount := balance.Sub(totalSend).Sub(actualFees)
	if changeAmount.GreaterThan(decimal.Zero) {
		outputAddrs = appendOutput(outputAddrs, changeAddress, changeAmount)
		//outputAddrs[changeAddress] = changeAmount.StringFixed(decoder.wm.Decimal())
	}

	tokenOutputAddrs = appendOutput(tokenOutputAddrs, toAddress, toAmount)
	//tokenOutputAddrs[toAddress] = toAmount

	if changeTokenAmount.GreaterThan(decimal.Zero) {
		tokenOutputAddrs = appendOutput(tokenOutputAddrs, changeAddress, changeTokenAmount)
	}

	err = decoder.createTokenRawTransaction(wrapper, rawTx, usedUTXO, outputAddrs, tokenOutputAddrs)
	if err != nil {
		return err
	}

	return nil
}

//CreateTokenSummaryRawTransaction 创建Token汇总交易
func (decoder *TransactionDecoder) CreateTokenSummaryRawTransaction(wrapper openwallet.WalletDAI, sumRawTx *openwallet.SummaryRawTransaction) ([]*openwallet.RawTransactionWithError, error) {

	var (
		feesRate            = decimal.New(0, 0)
		accountID           = sumRawTx.Account.AccountID
		minTransfer, _      = decimal.NewFromString(sumRawTx.MinTransfer)
		retainedBalance, _  = decimal.NewFromString(sumRawTx.RetainedBalance)
		rawTxArray          = make([]*openwallet.RawTransactionWithError, 0)
		outputAddrs         map[string]decimal.Decimal
		tokenOutputAddrs    map[string]decimal.Decimal
		feesSupportAccount  *openwallet.AssetsAccount
		feesSupportUnspents []*Unspent
	)

	// 如果有提供手续费账户，检查账户是否存在
	if feesAcount := sumRawTx.FeesSupportAccount; feesAcount != nil {
		account, supportErr := wrapper.GetAssetsAccountInfo(feesAcount.AccountID)
		if supportErr != nil {
			return nil, openwallet.Errorf(openwallet.ErrAccountNotFound, "can not find fees support account")
		}

		feesSupportAccount = account
		//查询可支持的utxo数组
		feesSupportUnspents, _ = decoder.getAssetsAccountUnspents(wrapper, feesSupportAccount)
	}

	if len(sumRawTx.Coin.Contract.Address) == 0 {
		return nil, fmt.Errorf("contract address is empty")
	}

	//Omni代币编号
	propertyID := sumRawTx.Coin.Contract.Address
	//tokenDecimals := int32(sumRawTx.Coin.Contract.Decimals)

	if minTransfer.LessThan(retainedBalance) {
		return nil, fmt.Errorf("mini transfer amount must be greater than address retained balance")
	}

	address, err := wrapper.GetAddressList(sumRawTx.AddressStartIndex, sumRawTx.AddressLimit, "AccountID", sumRawTx.Account.AccountID)
	if err != nil {
		return nil, err
	}

	if len(address) == 0 {
		return nil, fmt.Errorf("[%s] have not addresses", accountID)
	}

	//取得费率
	if len(sumRawTx.FeeRate) == 0 {
		feesRate, err = decoder.wm.EstimateFeeRate()
		if err != nil {
			return nil, err
		}
	} else {
		feesRate, _ = decimal.NewFromString(sumRawTx.FeeRate)
	}

	/*

		1. 遍历账户所有地址。
		2. 查询地址的token余额。
		3. 查询地址是否有utxo。（地址要有足够的主币做手续费）
		4. 保留余额，检查手续费是否足够
		5. 构建omni交易单。
		6. 把原始交易单加入到数组。

	*/

	for _, address := range address {

		//清空临时变量
		outputAddrs = make(map[string]decimal.Decimal, 0)
		tokenOutputAddrs = make(map[string]decimal.Decimal, 0)
		useUTXO := make([]*Unspent, 0)
		//decoder.wm.Log.Debug("address.Address:", address.Address)
		//查找地址token余额
		tokenBalance, createErr := decoder.wm.GetAssetsBalance(propertyID, address.Address)
		if createErr != nil {
			continue
		}

		//decoder.wm.Log.Debug("tokenBalance:", tokenBalance)
		//查询地址的utxo
		unspents, createErr := decoder.wm.ListUnspent(sumRawTx.Confirms, address.Address)
		if createErr != nil {
			continue
		}

		assetsUTXO, _, terr := decoder.wm.getAvailableUnspentByAssets(unspents, propertyID, tokenBalance)
		if terr != nil {
			continue
		}

		useUTXO = append(useUTXO, assetsUTXO...)

		cxcUTXO := decoder.wm.getCXCUnspent(unspents)

		useUTXO = append(useUTXO, cxcUTXO...)

		if tokenBalance.LessThan(minTransfer) || len(assetsUTXO) == 0 || tokenBalance.LessThanOrEqual(decimal.Zero) {
			continue
		}

		//合计地址主币余额
		addrBalance := decimal.Zero
		changeAmount := decimal.Zero

		for _, u := range useUTXO {
			if u.Spendable {
				ua, _ := decimal.NewFromString(u.Amount)
				addrBalance = addrBalance.Add(ua)
			}
		}
		//decoder.wm.Log.Debug("addrBalance:", addrBalance)
		//计算手续费，构建交易单inputs + 1（可选手续费地址），输出2个，1个为目标地址，1个为找零
		fees, createErr := decoder.wm.EstimateFee(int64(len(useUTXO))+1, 2, feesRate)
		if createErr != nil {
			return nil, createErr
		}

		totalCost := fees

		//地址的主币余额要，不足够最低转账成本+手续费
		if addrBalance.LessThan(totalCost) {

			//创建一笔交易单
			feeSupportRawTx := &openwallet.RawTransaction{
				Coin:    sumRawTx.Coin,
				Account: sumRawTx.Account,
			}

			//没有手续费账户支持，记录该交易单失败
			if feesSupportAccount == nil {
				rawTxWithErr := &openwallet.RawTransactionWithError{
					RawTx: feeSupportRawTx,
					Error: openwallet.Errorf(openwallet.ErrInsufficientFees, "address[%s] available %s: %s is less than totalCost: %s", address.Address, sumRawTx.Coin.Symbol, addrBalance.String(), totalCost.String()),
				}
				//添加到队列
				rawTxArray = append(rawTxArray, rawTxWithErr)
				continue
			}

			//查找足够付费的utxo
			supportUnspent, supportErr := decoder.getUTXOSatisfyAmount(feesSupportUnspents, totalCost)
			//supportUnspent, supportErr := decoder.getAssetsAccountUnspentSatisfyAmount(wrapper, feesSupportAccount, totalCost)
			if supportErr != nil {
				rawTxWithErr := &openwallet.RawTransactionWithError{
					RawTx: feeSupportRawTx,
					Error: supportErr,
				}
				//添加到队列
				rawTxArray = append(rawTxArray, rawTxWithErr)
				continue
			}

			//通过手续费账户创建交易单
			supportAddress := address.Address
			supportAmount, _ := decimal.NewFromString(supportUnspent.Amount)

			decoder.wm.Log.Debugf("create transaction for fees support account")
			decoder.wm.Log.Debugf("fees account: %s", feesSupportAccount.AccountID)
			decoder.wm.Log.Debugf("mini support amount: %s", totalCost.String())
			decoder.wm.Log.Debugf("allow support amount: %s", supportAmount.String())
			decoder.wm.Log.Debugf("support address: %s", supportAddress)

			//手续费地址utxo作为输入
			useUTXO = append(useUTXO, supportUnspent)

			//手续费地址 计算找零 = 手续费支持数量 + 地址余额 - 手续费 - 最低成本
			changeAmount = supportAmount.Add(addrBalance).Sub(totalCost)
			if changeAmount.GreaterThan(decimal.Zero) {
				//主币输出第二个地址为找零地址，找零主币
				outputAddrs = appendOutput(outputAddrs, supportUnspent.Address, changeAmount)
				//outputAddrs[supportUnspent.Address] = changeAmount.StringFixed(coinDecimals)
			}

			//删除已使用的utxo
			feesSupportUnspents = removeUTXO(feesSupportUnspents, supportUnspent)

		} else {

			//计算找零 = 地址余额 - 手续费 - 汇总地址的最低转账成本
			changeAmount = addrBalance.Sub(totalCost)
			if changeAmount.GreaterThan(decimal.Zero) {
				//主币输出第二个地址为找零地址，找零主币到汇总地址
				//outputAddrs = appendOutput(outputAddrs, address.Address, changeAmount)
				outputAddrs = appendOutput(outputAddrs, sumRawTx.SummaryAddress, changeAmount)
				//outputAddrs[address.Address] = changeAmount.StringFixed(coinDecimals)
			}

		}

		//主币输出第一个为汇总地址，把地址所有主币也汇总到汇总地址
		//outputAddrs = appendOutput(outputAddrs, sumRawTx.SummaryAddress, tokenBalance)
		//outputAddrs[sumRawTx.SummaryAddress] = transferCost.StringFixed(coinDecimals)

		//计算汇总数量
		//sumTokenAmount := tokenBalance.Sub(retainedBalance)
		//omni输出汇总地址及汇总数量
		tokenOutputAddrs = appendOutput(tokenOutputAddrs, sumRawTx.SummaryAddress, tokenBalance)
		//tokenOutputAddrs[sumRawTx.SummaryAddress] = sumTokenAmount.StringFixed(tokenDecimals)

		decoder.wm.Log.Debugf("tokenBalance: %v", tokenBalance)
		decoder.wm.Log.Debugf("addressBalance: %v", addrBalance)
		decoder.wm.Log.Debugf("fees: %v", fees)
		decoder.wm.Log.Debugf("changeAmount: %v", changeAmount)
		decoder.wm.Log.Debugf("sumTokenAmount: %v", tokenBalance)

		//创建一笔交易单
		rawTx := &openwallet.RawTransaction{
			Coin:     sumRawTx.Coin,
			Account:  sumRawTx.Account,
			FeeRate:  sumRawTx.FeeRate,
			To:       map[string]string{sumRawTx.SummaryAddress: tokenBalance.String()},
			Fees:     fees.StringFixed(decoder.wm.Decimal()),
			Required: 1,
		}

		createTxErr := decoder.createTokenRawTransaction(wrapper, rawTx, useUTXO, outputAddrs, tokenOutputAddrs)
		rawTxWithErr := &openwallet.RawTransactionWithError{
			RawTx: rawTx,
			Error: openwallet.ConvertError(createTxErr),
		}

		//创建成功，添加到队列
		rawTxArray = append(rawTxArray, rawTxWithErr)

	}

	return rawTxArray, nil
}

//createTokenRawTransaction 创建token原始交易单
func (decoder *TransactionDecoder) createTokenRawTransaction(
	wrapper openwallet.WalletDAI,
	rawTx *openwallet.RawTransaction,
	usedUTXO []*Unspent,
	coinTo map[string]decimal.Decimal,
	tokenTo map[string]decimal.Decimal,
) error {

	var (
		err              error
		vins             = make([]cxcTransaction.Vin, 0)
		vout             cxcTransaction.Vout
		txUnlocks        = make([]cxcTransaction.TxUnlock, 0)
		tokenOutput      = make([]cxcTransaction.AssetTransfer, 0)
		accountTotalSent = decimal.Zero
		toAmount         = decimal.Zero
		txFrom           = make([]string, 0)
		txTo             = make([]string, 0)
		accountID        = rawTx.Account.AccountID
		addressPrefix    cxcTransaction.AddressPrefix
	)

	if len(usedUTXO) == 0 {
		return fmt.Errorf("utxo is empty")
	}

	if len(coinTo) == 0 {
		return fmt.Errorf("Receiver addresses is empty! ")
	}

	if len(tokenTo) == 0 {
		return fmt.Errorf("Receiver addresses is empty! ")
	}

	//Token代币编号
	propertyID := rawTx.Coin.Contract.Address
	tokenDecimals := int32(rawTx.Coin.Contract.Decimals)
	assetsInfo, err := decoder.wm.ShowAssets(propertyID)
	if err != nil {
		return err
	}

	selltxidhalf := assetsInfo.Selltxid[:len(assetsInfo.Selltxid)/2]

	//记录输入输出明细
	for addr, amount := range tokenTo {
		//接收方的地址和数量
		txTo = append(txTo, fmt.Sprintf("%s:%s", addr, amount.Shift(tokenDecimals)))
		//toAmount, _ = decimal.NewFromString(amount)
		//计算账户的实际转账amount
		addresses, findErr := wrapper.GetAddressList(0, -1, "AccountID", accountID, "Address", addr)
		if findErr != nil || len(addresses) == 0 {
			accountTotalSent = accountTotalSent.Add(toAmount)
		}

		tokenAmount := amount.Shift(tokenDecimals)
		tokenDetail := cxcTransaction.AssetTransfer{
			FirstSellTxID: selltxidhalf,
			Address:       addr,
			Amount:        uint64(tokenAmount.IntPart()),
		}

		tokenOutput = append(tokenOutput, tokenDetail)
	}

	//UTXO如果大于设定限制，则分拆成多笔交易单发送
	if len(usedUTXO) > decoder.wm.Config.MaxTxInputs {
		errStr := fmt.Sprintf("The transaction is use max inputs over: %d", decoder.wm.Config.MaxTxInputs)
		return errors.New(errStr)
	}

	//装配输入
	for _, utxo := range usedUTXO {
		in := cxcTransaction.Vin{utxo.TxID, uint32(utxo.Vout)}
		vins = append(vins, in)

		txUnlock := cxcTransaction.TxUnlock{LockScript: utxo.ScriptPubKey, SigType: cxcTransaction.SigHashAll}
		txUnlocks = append(txUnlocks, txUnlock)

		for _, as := range utxo.Assets {
			if as.Assetref == propertyID {
				amount, _ := decimal.NewFromString(as.Qty)
				txFrom = append(txFrom, fmt.Sprintf("%s:%s", utxo.Address, amount.Shift(tokenDecimals)))
			}
		}
	}

	//装配输入
	for to, amount := range coinTo {
		amount = amount.Shift(decoder.wm.Decimal())
		vout = cxcTransaction.Vout{to, uint64(amount.IntPart())}
		//txTo = append(txTo, fmt.Sprintf("%s:%s", to, amount))
	}

	if decoder.wm.Config.IsTestNet {
		addressPrefix = cxcTransaction.AddressPrefix{
			P2PKHPrefix:  decoder.wm.Config.TestNetAddressPrefix.P2PKHPrefix,
			P2WPKHPrefix: decoder.wm.Config.TestNetAddressPrefix.P2WPKHPrefix,
			Bech32Prefix: decoder.wm.Config.TestNetAddressPrefix.Bech32Prefix,
		}
	} else {
		addressPrefix = cxcTransaction.AddressPrefix{
			P2PKHPrefix:  decoder.wm.Config.MainNetAddressPrefix.P2PKHPrefix,
			P2WPKHPrefix: decoder.wm.Config.MainNetAddressPrefix.P2WPKHPrefix,
			Bech32Prefix: decoder.wm.Config.MainNetAddressPrefix.Bech32Prefix,
		}
	}

	//锁定时间
	lockTime := uint32(0)

	//追加手续费支持
	replaceable := false

	/////////构建空交易单
	emptyTrans, err := cxcTransaction.CreateEmptyAssetRawTransaction(vins, tokenOutput, vout, lockTime, replaceable, addressPrefix)

	if err != nil {
		return fmt.Errorf("create transaction failed, unexpected error: %v", err)
		//decoder.wm.Log.Error("构建空交易单失败")
	}

	////////构建用于签名的交易单哈希
	transHash, err := cxcTransaction.CreateRawTransactionHashForSig(emptyTrans, txUnlocks, decoder.wm.Config.SupportSegWit, addressPrefix)
	if err != nil {
		return fmt.Errorf("create transaction hash for sig failed, unexpected error: %v", err)
		//decoder.wm.Log.Error("获取待签名交易单哈希失败")
	}

	rawTx.RawHex = emptyTrans

	signatures := rawTx.Signatures
	if signatures == nil {
		signatures = make(map[string][]*openwallet.KeySignature)
	}

	for i, txHash := range transHash {

		var unlockAddr string

		//txHash := transHash[i]

		//判断是否是多重签名
		if txHash.IsMultisig() {
			//获取地址
			//unlockAddr = txHash.GetMultiTxPubkeys() //返回hex数组
		} else {
			//获取地址
			unlockAddr = txHash.GetNormalTxAddress() //返回hex串
		}
		//获取hash值
		beSignHex := txHash.GetTxHashHex()

		decoder.wm.Log.Std.Debug("txHash[%d]: %s", i, beSignHex)
		//beSignHex := transHash[i]

		addr, err := wrapper.GetAddress(unlockAddr)
		if err != nil {
			return err
		}

		signature := &openwallet.KeySignature{
			EccType: decoder.wm.Config.CurveType,
			Nonce:   "",
			Address: addr,
			Message: beSignHex,
		}

		keySigs := signatures[addr.AccountID]
		if keySigs == nil {
			keySigs = make([]*openwallet.KeySignature, 0)
		}

		//装配签名
		keySigs = append(keySigs, signature)

		signatures[addr.AccountID] = keySigs
	}

	//feesDec, _ := decimal.NewFromString(rawTx.Fees)
	//accountTotalSent = accountTotalSent.Add(feesDec)
	accountTotalSent = decimal.Zero.Sub(accountTotalSent)

	rawTx.Signatures = signatures
	rawTx.IsBuilt = true
	rawTx.TxAmount = accountTotalSent.StringFixed(tokenDecimals)
	rawTx.TxFrom = txFrom
	rawTx.TxTo = txTo

	return nil
}

// CreateSummaryRawTransactionWithError 创建汇总交易，返回能原始交易单数组（包含带错误的原始交易单）
func (decoder *TransactionDecoder) CreateSummaryRawTransactionWithError(wrapper openwallet.WalletDAI, sumRawTx *openwallet.SummaryRawTransaction) ([]*openwallet.RawTransactionWithError, error) {
	if sumRawTx.Coin.IsContract {
		return decoder.CreateTokenSummaryRawTransaction(wrapper, sumRawTx)
	} else {
		return decoder.CreateBTCSummaryRawTransaction(wrapper, sumRawTx)
	}
}

// getAssetsAccountUnspentSatisfyAmount
func (decoder *TransactionDecoder) getAssetsAccountUnspents(wrapper openwallet.WalletDAI, account *openwallet.AssetsAccount) ([]*Unspent, *openwallet.Error) {

	address, err := wrapper.GetAddressList(0, -1, "AccountID", account.AccountID)
	if err != nil {
		return nil, openwallet.Errorf(openwallet.ErrAccountNotAddress, err.Error())
	}

	if len(address) == 0 {
		return nil, openwallet.Errorf(openwallet.ErrAccountNotAddress, "[%s] have not addresses", account.AccountID)
	}

	searchAddrs := make([]string, 0)
	for _, address := range address {
		searchAddrs = append(searchAddrs, address.Address)
	}
	//decoder.wm.Log.Debug(searchAddrs)
	//查找账户的utxo
	unspents, err := decoder.wm.ListUnspent(0, searchAddrs...)
	if err != nil {
		return nil, openwallet.Errorf(openwallet.ErrCallFullNodeAPIFailed, err.Error())
	}

	return unspents, nil
}

// getAssetsAccountUnspentSatisfyAmount
func (decoder *TransactionDecoder) getUTXOSatisfyAmount(unspents []*Unspent, amount decimal.Decimal) (*Unspent, *openwallet.Error) {

	var utxo *Unspent

	if unspents != nil {
		for _, u := range unspents {
			if u.Spendable {
				ua, _ := decimal.NewFromString(u.Amount)
				if ua.GreaterThanOrEqual(amount) {
					utxo = u
					break
				}
			}
		}
	}

	if utxo == nil {
		return nil, openwallet.Errorf(openwallet.ErrInsufficientBalanceOfAccount, "account have not available utxo")
	}

	return utxo, nil
}

// removeUTXO
func removeUTXO(slice []*Unspent, elem *Unspent) []*Unspent {
	if len(slice) == 0 {
		return slice
	}
	for i, v := range slice {
		if v == elem {
			slice = append(slice[:i], slice[i+1:]...)
			return removeUTXO(slice, elem)
			break
		}
	}
	return slice
}

func appendOutput(output map[string]decimal.Decimal, address string, amount decimal.Decimal) map[string]decimal.Decimal {
	if origin, ok := output[address]; ok {
		origin = origin.Add(amount)
		output[address] = origin
	} else {
		output[address] = amount
	}
	return output
}

//根据交易输入地址顺序重排交易hash
func resetTransHashFunc(origins []cxcTransaction.TxHash, addr string, start int) []cxcTransaction.TxHash {
	newHashs := make([]cxcTransaction.TxHash, start)
	copy(newHashs, origins[:start])
	end := 0
	for i := start; i < len(origins); i++ {
		h := origins[i]
		txAddress := h.GetNormalTxAddress()
		if txAddress == addr {
			newHashs = append(newHashs, h)
			end = i
			break
		}
	}

	newHashs = append(newHashs, origins[start:end]...)
	newHashs = append(newHashs, origins[end+1:]...)
	return newHashs
}
