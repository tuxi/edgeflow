package rest

import (
	"bytes"
	"context"
	"edgeflow/pkg/hype/types"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type HyperliquidRestClient struct {
	url            string
	leaderboardURL string
	httpClient     *http.Client
}

func NewHyperliquidRestClient(rawUrl string, leaderboardUrl string) (*HyperliquidRestClient, error) {
	urls := []string{rawUrl, leaderboardUrl}
	parsedUrls := make([]string, len(urls))

	for i, inputUrl := range urls {
		parsedUrl, err := url.Parse(inputUrl)
		if err != nil || parsedUrl.Scheme == "" || parsedUrl.Host == "" {
			return nil, fmt.Errorf("invalid URL: %s", inputUrl)
		}
		if len(parsedUrl.Path) > 0 && parsedUrl.Path[len(parsedUrl.Path)-1:] == "/" {
			parsedUrl.Path = parsedUrl.Path[:len(parsedUrl.Path)-1]
		}
		parsedUrls[i] = parsedUrl.String()
	}

	return &HyperliquidRestClient{
		url:            parsedUrls[0],
		leaderboardURL: parsedUrls[1],
		httpClient:     &http.Client{},
	}, nil
}

func (rest *HyperliquidRestClient) doRequestWithContext(ctx context.Context, endpoint string, requestType string, additionalParams map[string]interface{}, result interface{}) error {
	reqBody := map[string]interface{}{"type": requestType}
	for key, value := range additionalParams {
		reqBody[key] = value
	}
	reqBodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", rest.url+endpoint, bytes.NewBuffer(reqBodyJSON))
	if err != nil {
		return fmt.Errorf("failed to create new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := rest.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-OK HTTP status: %s", resp.Status)
	}

	byteData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(byteData, result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return nil
}

func (rest *HyperliquidRestClient) doRequest(endpoint string, requestType string, additionalParams map[string]interface{}, result interface{}) error {
	return rest.doRequestWithContext(context.Background(), endpoint, requestType, additionalParams, rest)
}

// New method for fetching metadata
func (rest *HyperliquidRestClient) PerpetualsMetadata() (types.Universe, error) {
	var metadata types.Universe
	if err := rest.doRequest("/info", "meta", nil, &metadata); err != nil {
		return types.Universe{}, err
	}
	return metadata, nil
}

// Updated PerpetualAssetContexts function
func (rest *HyperliquidRestClient) PerpetualAssetContexts() ([]types.UniverseItem, []types.AssetContext, error) {
	var respData []json.RawMessage
	if err := rest.doRequest("/info", "metaAndAssetCtxs", nil, &respData); err != nil {
		return nil, nil, err
	}

	// Parse universe part
	var universeData types.Universe
	if err := json.Unmarshal(respData[0], &universeData); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal universe data: %w", err)
	}

	// Parse asset contexts part
	var assetContexts []types.AssetContext
	if err := json.Unmarshal(respData[1], &assetContexts); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal asset contexts: %w", err)
	}

	return universeData.Universe, assetContexts, nil
}

// 获取账号信息
// New method for fetching a user's perpetuals account summary
func (rest *HyperliquidRestClient) PerpetualsAccountSummary(ctx context.Context, user string) (types.MarginData, error) {
	var marginData types.MarginData
	params := map[string]interface{}{"user": user}
	if err := rest.doRequestWithContext(ctx, "/info", "clearinghouseState", params, &marginData); err != nil {
		return types.MarginData{}, err
	}
	return marginData, nil
}

// 获取用户24小时内的成交历史
func (rest *HyperliquidRestClient) UserFillOrdersIn24Hours(ctx context.Context, userAddress string) (orders []*types.UserFillOrder, err error) {

	// 只查询获取24小时内的交易记录
	// 只调用一次 time.Now()，避免两次调用间的时间差
	now := time.Now()
	startTime := now.Add(-24 * time.Hour).UnixMilli()
	//endTime := now.UnixMilli()

	params := map[string]interface{}{
		"user":      userAddress,
		"startTime": startTime,
		//"endTime":   endTime,
	}
	if err := rest.doRequestWithContext(ctx, "/info", "userFillsByTime", params, &orders); err != nil {
		return nil, err
	}
	return orders, nil
}

func (rest *HyperliquidRestClient) UserOpenOrders(ctx context.Context, userAddress string) (orders []*types.UserOpenOrder, err error) {

	params := map[string]interface{}{
		"user": userAddress,
	}
	if err := rest.doRequestWithContext(ctx, "/info", "frontendOpenOrders", params, &orders); err != nil {
		return nil, err
	}

	return orders, nil
}

// 获取用户非资金分类账更新包括存款、转账和提款
func (rest *HyperliquidRestClient) UserNonFundingLedgerGet(ctx context.Context, userAddress string) (orders []*types.UserNonFunding, err error) {

	// 只查询获取24小时内的交易记录
	// 只调用一次 time.Now()，避免两次调用间的时间差
	now := time.Now()
	startTime := now.Add(-24 * time.Hour).UnixMilli()
	//endTime := now.UnixMilli()

	params := map[string]interface{}{
		"user":      userAddress,
		"startTime": startTime,
		//"endTime":   endTime,
	}
	if err := rest.doRequestWithContext(ctx, "/info", "userNonFundingLedgerUpdates", params, &orders); err != nil {
		return nil, err
	}
	return orders, nil
}

// 获取收益率排行榜数据
func (rest *HyperliquidRestClient) LeaderboardCall() ([]types.TraderPerformance, error) {
	req, err := http.NewRequest("GET", rest.leaderboardURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := rest.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch leaderboard: %v", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rawResponse types.RawLeaderboardResponse
	if err := json.Unmarshal(body, &rawResponse); err != nil {
		return nil, err
	}

	traderPerformances := make([]types.TraderPerformance, len(rawResponse.LeaderboardRows))
	for i, item := range rawResponse.LeaderboardRows {
		accountValue := parseStringToFloat(item.AccountValue)
		windowPerformances := item.WindowPerformances
		traderPerformance := types.TraderPerformance{
			EthAddress:  item.EthAddress,
			Prize:       item.Prize,
			DisplayName: item.DisplayName,
			Day: types.PeriodPerformance{
				Pnl: parseStringToFloat(windowPerformances[0][1].(map[string]interface{})["pnl"].(string)),
				Roi: parseStringToFloat(windowPerformances[0][1].(map[string]interface{})["roi"].(string)),
				Vlm: parseStringToFloat(windowPerformances[0][1].(map[string]interface{})["vlm"].(string)),
			},
			Week: types.PeriodPerformance{
				Pnl: parseStringToFloat(windowPerformances[1][1].(map[string]interface{})["pnl"].(string)),
				Roi: parseStringToFloat(windowPerformances[1][1].(map[string]interface{})["roi"].(string)),
				Vlm: parseStringToFloat(windowPerformances[1][1].(map[string]interface{})["vlm"].(string)),
			},
			Month: types.PeriodPerformance{
				Pnl: parseStringToFloat(windowPerformances[2][1].(map[string]interface{})["pnl"].(string)),
				Roi: parseStringToFloat(windowPerformances[2][1].(map[string]interface{})["roi"].(string)),
				Vlm: parseStringToFloat(windowPerformances[2][1].(map[string]interface{})["vlm"].(string)),
			},
			AllTime: types.PeriodPerformance{
				Pnl: parseStringToFloat(windowPerformances[3][1].(map[string]interface{})["pnl"].(string)),
				Roi: parseStringToFloat(windowPerformances[3][1].(map[string]interface{})["roi"].(string)),
				Vlm: parseStringToFloat(windowPerformances[3][1].(map[string]interface{})["vlm"].(string)),
			},
			AccountValue: accountValue,
		}
		traderPerformances[i] = traderPerformance
	}
	return traderPerformances, nil
}
