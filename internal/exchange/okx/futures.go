package okx

import (
	"edgeflow/internal/account"
	goexv2 "github.com/nntaoli-project/goex/v2"
	"github.com/nntaoli-project/goex/v2/okx/futures"
	"github.com/nntaoli-project/goex/v2/options"
)

// 交割合约
type OkxFutures struct {
	FuturesCommon
	pub futures.Futures
}

func NewOkxFutures(conf []options.ApiOption) *OkxFutures {
	pub := goexv2.OKx.Futures
	return &OkxFutures{
		FuturesCommon: FuturesCommon{Okx{
			prv:     pub.NewPrvApi(conf...),
			Account: account.NewAccountService(pub.NewPrvApi(conf...)),
			pub:     pub,
		}},
		pub: *pub,
	}
}

func (e *OkxFutures) getPub() goexv2.IPubRest {
	return &e.pub
}
