package currency

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/service"
	"edgeflow/pkg/errors"
	"edgeflow/pkg/errors/ecode"
	"edgeflow/pkg/response"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"os"
	"strconv"
)

type Handler struct {
	service service.CurrencyService
}

func NewHandler(service service.CurrencyService) *Handler {
	handler := &Handler{service: service}

	// 初始化交易所和几个主流coins数据
	handler.initCurrenciesData()
	return handler
}

func (h *Handler) CoinsGetList() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.CurrencyListReq
		if err := ctx.ShouldBindQuery(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		exId, err := strconv.ParseInt(req.ExId, 10, 64)
		if err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, "exchancge id转换错误"), nil)
			return
		}

		// 校验参数
		if req.Limit < 0 {
			req.Limit = 2
		}
		if req.Page < 1 {
			req.Page = 1
		}

		res, err := h.service.CurrencyListGetByExchange(ctx, exId, req.Page, req.Limit)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.Unknown, "接口调用失败"), nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}

func (h *Handler) initCurrenciesData() {
	files := []string{"coins.json"}

	// 初始化OKX和币安交易所
	_ = h.service.ExchangeCreateNew(context.Background(), "OKX", "OKX")
	_ = h.service.ExchangeCreateNew(context.Background(), "币安", "Binance")

	for _, f := range files {
		data, err := os.ReadFile(fmt.Sprintf("conf/%s", f))
		if err != nil {
			return
		}

		var tempData = struct {
			List []struct {
				Abbreviation string `json:"abbreviation"`
				PriceScale   int    `json:"priceScale"`
				Icon         string `json:"icon"`
				NameZh       string `json:"nameZh"`
				NameEn       string `json:"nameEn"`
			} `json:"list"`
		}{}

		if err := json.Unmarshal(data, &tempData); err != nil {
			return
		}

		var count = 0
		var ids []int64
		for _, item := range tempData.List {
			res, err := h.service.CurrencyCreateNew(context.Background(), item.Abbreviation, item.NameZh, item.NameEn, item.PriceScale, item.Icon)
			if err != nil {
				continue
			}
			count += 1
			id, _ := strconv.ParseInt(res.CcyId, 10, 64)
			ids = append(ids, id)
		}
		log.Printf("一共%v条数据，创建成功%v条", len(tempData.List), count)

		// 上线交易所
		err = h.service.AssociateCurrenciesWithExchangeBatch(context.Background(), ids, 1)
		if err != nil {
			log.Println("批量上线okx交易所失败")
		}
	}
}
