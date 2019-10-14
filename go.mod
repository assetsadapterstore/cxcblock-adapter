module github.com/assetsadapterstore/cxcblock-adapter

go 1.12

require (
	github.com/asdine/storm v2.1.2+incompatible
	github.com/astaxie/beego v1.11.1
	github.com/blocktree/go-owcdrivers v1.1.16
	github.com/blocktree/go-owcrypt v1.0.3
	github.com/blocktree/openwallet v1.4.8
	github.com/btcsuite/btcd v0.0.0-20190315201642-aa6e0f35703c
	github.com/imroc/req v0.2.3
	github.com/pborman/uuid v1.2.0
	github.com/shopspring/decimal v0.0.0-20180709203117-cd690d0c9e24
	github.com/tidwall/gjson v1.2.1
)

//replace github.com/blocktree/go-owcdrivers => ../../go-owcdrivers
