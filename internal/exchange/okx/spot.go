package okx

import (
	"edgeflow/internal/account"
	goexv2 "github.com/nntaoli-project/goex/v2"
	"github.com/nntaoli-project/goex/v2/okx/spot"
	"github.com/nntaoli-project/goex/v2/options"
)

// 现货
type OkxSpot struct {
	Okx
	pub spot.Spot
}

func NewOkxSpot(conf []options.ApiOption) *OkxSpot {
	pub := goexv2.OKx.Spot
	return &OkxSpot{
		Okx: Okx{
			prv:     pub.NewPrvApi(conf...),
			Account: account.NewAccountService(pub.NewPrvApi(conf...)),
			pub:     pub,
		},
		pub: *pub,
	}
}

func (e *OkxSpot) getPub() goexv2.IPubRest {
	return &e.pub
}
