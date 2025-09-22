package coin

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
	service service.CoinService
}

func NewHandler(service service.CoinService) *Handler {
	handler := &Handler{service: service}

	// 初始化coins数据
	//handler.initCoinsData()
	return handler
}

func (h *Handler) CoinsGetList() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.CoinListReq
		if err := ctx.ShouldBindQuery(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
		}

		caregoryId, err := strconv.ParseInt(req.CategoryId, 10, 64)
		if err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, "category id转换错误"), nil)
			return
		}

		// 校验参数
		if req.Limit < 0 {
			req.Limit = 2
		}
		if req.Page < 1 {
			req.Page = 1
		}

		res, err := h.service.CoinListGetByCategory(ctx, caregoryId, req.Page, req.Limit)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.Unknown, "接口调用失败"), nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}

func (h *Handler) initCoinsData() {
	files := []string{"coins1.json", "coins2.json", "coins3.json", "coins4.json", "coins5.json", "coins6.json", "coins7.json"}
	for _, f := range files {
		data, err := os.ReadFile(fmt.Sprintf("conf/%s", f))
		if err != nil {
			return
		}

		var tempData = struct {
			Data struct {
				List []struct {
					Abbreviation string `json:"abbreviation"`
					PriceScale   int    `json:"priceScale"`
					Icon         string `json:"icon"`
					NameZh       string `json:"nameZh"`
					NameEn       string `json:"nameEn"`
				} `json:"list"`
			} `json:"data"`
		}{}

		if err := json.Unmarshal(data, &tempData); err != nil {
			return
		}

		var count = 0
		for _, item := range tempData.Data.List {
			_, err := h.service.CoinCreateNew(context.Background(), item.Abbreviation, item.NameZh, item.NameEn, 1, item.PriceScale, item.Icon)
			if err != nil {
				continue
			}
			count += 1
		}
		log.Printf("一共%v条数据，创建成功%v条", len(tempData.Data.List), count)
	}
}
