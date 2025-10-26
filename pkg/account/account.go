package account

import (
	"context"
	"errors"
	goexv2 "github.com/nntaoli-project/goex/v2"
	"github.com/nntaoli-project/goex/v2/model"
	"time"
)

type Service struct {
	prv goexv2.IPrvRest
}

// NewAccountService 创建账户服务，prv是goex私有API客户端
func NewAccountService(prv goexv2.IPrvRest) *Service {
	return &Service{prv: prv}
}

// GetBalance 查询指定币种的账户余额（可用余额）
func (s *Service) GetAccount(ctx context.Context, coin string) (account *Account, err error) {
	// goex私有方法没有context，临时用超时控制
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	ch := make(chan struct {
		bal map[string]model.Account
		err error
	})

	go func() {
		bal, _, err := s.prv.GetAccount(coin)
		ch <- struct {
			bal map[string]model.Account
			err error
		}{bal, err}
	}()

	select {
	case <-timeoutCtx.Done():
		return nil, timeoutCtx.Err()
	case result := <-ch:
		if result.err != nil {
			return nil, result.err
		}
		account, ok := result.bal[coin]
		if !ok {
			return nil, errors.New("account info not found for coin " + coin)
		}
		return &Account{
			Currency:  account.Coin,
			Total:     account.Balance,
			Available: account.AvailableBalance,
			Frozen:    account.FrozenBalance,
		}, nil
	}
}
