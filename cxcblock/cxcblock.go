package cxcblock

import (
	"github.com/astaxie/beego/config"
	"github.com/blocktree/openwallet/log"
	"github.com/blocktree/openwallet/openwallet"
	"github.com/shopspring/decimal"
)

//FullName 币种全名
func (wm *WalletManager) FullName() string {
	return "cxcblock"
}

//CurveType 曲线类型
func (wm *WalletManager) CurveType() uint32 {
	return wm.Config.CurveType
}

//Symbol 币种标识
func (wm *WalletManager) Symbol() string {
	return wm.Config.Symbol
}

//小数位精度
func (wm *WalletManager) Decimal() int32 {
	return wm.Config.Decimals
}

//AddressDecode 地址解析器
func (wm *WalletManager) GetAddressDecode() openwallet.AddressDecoder {
	return wm.Decoder
}

//TransactionDecoder 交易单解析器
func (wm *WalletManager) GetTransactionDecoder() openwallet.TransactionDecoder {
	return wm.TxDecoder
}

//GetBlockScanner 获取区块链
func (wm *WalletManager) GetBlockScanner() openwallet.BlockScanner {

	//先加载是否有配置文件
	//err := wm.LoadConfig()
	//if err != nil {
	//	return nil
	//}

	return wm.Blockscanner
}

//LoadAssetsConfig 加载外部配置
func (wm *WalletManager) LoadAssetsConfig(c config.Configer) error {

	wm.Config.RPCServerType, _ = c.Int("rpcServerType")
	wm.Config.ServerAPI = c.String("serverAPI")
	wm.Config.RpcUser = c.String("rpcUser")
	wm.Config.RpcPassword = c.String("rpcPassword")
	wm.Config.IsTestNet, _ = c.Bool("isTestNet")
	wm.Config.SupportSegWit, _ = c.Bool("supportSegWit")
	wm.Config.OmniTransferCost = c.String("omniTransferCost")
	wm.Config.OmniCoreAPI = c.String("omniCoreAPI")
	wm.Config.OmniRPCUser = c.String("omniRPCUser")
	wm.Config.OmniRPCPassword = c.String("omniRPCPassword")
	wm.Config.OmniSupport, _ = c.Bool("omniSupport")
	wm.Config.MinFees, _ = decimal.NewFromString(c.String("minFees"))
	wm.Config.MinFees = wm.Config.MinFees.Round(wm.Decimal())
	wm.Config.DataDir = c.String("dataDir")

	//数据文件夹
	wm.Config.makeDataDir()

	token := BasicAuth(wm.Config.RpcUser, wm.Config.RpcPassword)
	//omniToken := BasicAuth(wm.Config.OmniRPCUser, wm.Config.OmniRPCPassword)

	if wm.Config.RPCServerType == RPCServerCore {
		wm.WalletClient = NewClient(wm.Config.ServerAPI, token, false)
	} else {
		//wm.ExplorerClient = NewExplorer(wm.Config.ServerAPI, false)
	}

	//wm.OnmiClient = NewClient(wm.Config.OmniCoreAPI, omniToken, false)

	return nil
}

//InitAssetsConfig 初始化默认配置
func (wm *WalletManager) InitAssetsConfig() (config.Configer, error) {
	return config.NewConfigData("ini", []byte(wm.Config.DefaultConfig))
}

//GetAssetsLogger 获取资产账户日志工具
func (wm *WalletManager) GetAssetsLogger() *log.OWLogger {
	return wm.Log
}

//GetSmartContractDecoder 获取智能合约解析器
func (wm *WalletManager) GetSmartContractDecoder() openwallet.SmartContractDecoder {
	return wm.ContractDecoder
}