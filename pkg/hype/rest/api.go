package rest

import (
	"bytes"
	"context"
	"edgeflow/pkg/hype/types"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
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

	// 引入重试循环和指数退避
	const maxRetries = 5                // 最大重试次数
	const backoffBase = 5 * time.Second // 退避基数 2 秒
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {

		// 检查 Context 是否已被取消
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// 构建请求 (需要在循环内重新创建请求体和请求对象)
		// 为什么重新构建：部分 HTTP 客户端在请求失败或读取完 Body 后，可能无法重用 io.Reader (bytes.NewBuffer)。
		req, err := http.NewRequestWithContext(ctx, "POST", rest.url+endpoint, bytes.NewBuffer(reqBodyJSON))
		if err != nil {
			return fmt.Errorf("failed to create new request: %w", err) // 无法恢复的错误，立即返回
		}
		req.Header.Set("Content-Type", "application/json")

		// 2b. 执行请求
		resp, err := rest.httpClient.Do(req)

		if err != nil {
			lastErr = fmt.Errorf("failed to execute request (network error): %w", err)
			goto Retry // 网络错误，尝试重试
		}
		defer resp.Body.Close() // 必须在每次循环内关闭 Body

		// 检查状态码
		if resp.StatusCode == http.StatusOK {
			// 成功：读取数据并返回
			byteData, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response body: %w", err)
			}
			if err := json.Unmarshal(byteData, result); err != nil {
				return fmt.Errorf("failed to unmarshal response: %w", err)
			}
			return nil // 成功退出
		}

		// 处理 429 错误：需要重试
		if resp.StatusCode == 429 {
			lastErr = fmt.Errorf("received 429 Too Many Requests on attempt %d", attempt+1)
			goto Retry
		}

		// 2e. 处理其他非 OK 错误：通常不可恢复
		return fmt.Errorf("received non-OK HTTP status: %s", resp.Status)

		// --- 重试标签 ---
	Retry:
		if attempt == maxRetries-1 {
			// 达到最大重试次数，返回最后一次错误
			return fmt.Errorf("API failed after %d retries. Last error: %w", maxRetries, lastErr)
		}

		// 计算指数退避时间： backoffBase * 2^attempt
		waitTime := backoffBase * time.Duration(1<<attempt)
		//log.Printf("HyperliquidRestClient Retrying API request for type %s after %v due to error: %v", requestType, waitTime, lastErr)

		// 暂停等待
		select {
		case <-time.After(waitTime):
			// 等待结束，进入下一次循环
		case <-ctx.Done():
			// Context 被取消，立即退出
			return ctx.Err()
		}
	}
	// 代码不会到达这里，但为了完整性，返回一个错误
	return fmt.Errorf("unexpected exit from retry loop")

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

// Hyperliquid 的账户持仓分析接口（type: "portfolio"），返回指定用户的账户资产历史和盈亏历史
// 获取指定用户在 Hyperliquid 平台的账户资金变化accountValueHistory、盈亏历史pnlHistory、交易量vlm等数据（从天、周、月、全部时间四个维度统计
func (rest *HyperliquidRestClient) WhalePortfolioInfo(ctx context.Context, address string) (*types.WhalePortfolio, error) {
	reqBody := map[string]interface{}{"type": "portfolio", "user": address}
	reqBodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", rest.url+"/info", bytes.NewBuffer(reqBodyJSON))
	req.Header.Set("Content-Type", "application/json")
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

	portfolio, err := parsePortfolio(body)
	if err != nil {
		return nil, err
	}
	fmt.Println("=== 总账户 (Total) ===")
	fmt.Printf("Day PnL: %.2f\n", lastValue(portfolio.Total.Day.Pnl))
	fmt.Printf("Day Account Value: %.2f\n", lastValue(portfolio.Total.Day.AccountValue))
	fmt.Println("=== 合约账户 (Perp) ===")
	fmt.Printf("PerpDay PnL: %.2f\n", lastValue(portfolio.Perp.Day.Pnl))
	fmt.Printf("PerpDay Account Value: %.2f\n", lastValue(portfolio.Perp.Day.AccountValue))
	return portfolio, nil
}

func lastValue(arr []types.DataPoint) float64 {
	if len(arr) == 0 {
		return 0
	}
	return arr[len(arr)-1].Value
}

// 获取账号信息: 主要包含永续合约持仓、盈亏、资金费
// New method for fetching a user's perpetuals account summary
func (rest *HyperliquidRestClient) PerpetualsAccountSummary(ctx context.Context, user string) (types.MarginData, error) {
	var marginData types.MarginData
	params := map[string]interface{}{"user": user}
	if err := rest.doRequestWithContext(ctx, "/info", "clearinghouseState", params, &marginData); err != nil {
		return types.MarginData{}, err
	}
	return marginData, nil
}

const (
	// 单次查询的默认窗口大小：3 小时
	DefaultPullWindow = 3 * time.Hour
	// 假设 Hyperliquid API 单次最多返回 2000 条记录
	APIRecordLimit = 2000

	// 只拉取 since 到 now 的增量数据，并按 1 小时分块。
	MaxQueryChunk = 3 * time.Hour // 初始查询窗口大小 (3小时)

	// 最小和最大拉取窗口
	MinPullWindow = 1 * time.Hour  // 即使在爆发期，最小也拉取 1 小时
	MaxPullWindow = 12 * time.Hour // 在稀疏期，最大拉取 12 小时（或根据 API 限制调整）
)

// UserFillOrdersIn24Hours：实现客户端可追溯的分页查询
func (rest *HyperliquidRestClient) UserFillOrdersIn24Hours( // 客户端可追溯的分页查询，支持动态窗口
	ctx context.Context,
	userAddress string,  // 鲸鱼地址
	since int64,         // 上次查询返回的起始时间戳（客户端传回）。如果为 0，则默认查询 [now - 3h, now]。
	maxLookbackDays int, // 最大追溯天数，默认1天
	prevWindowHours int, // 上次查询使用的窗口大小（小时）。如果为 0，使用默认窗口。
) (data *types.UserFillOrderData, err error) {
	// 确保追溯天数有效
	if maxLookbackDays <= 0 {
		maxLookbackDays = 1
	}

	// 1. 确定本次查询的窗口大小 (Duration) - 优先使用传入的 prevWindowHours
	var currentWindow time.Duration
	if prevWindowHours <= 0 {
		// 首次查询或客户端未提供，使用默认值
		currentWindow = DefaultPullWindow // 假设 DefaultPullWindow = 3 * time.Hour
	} else {
		// 将传入的小时数转换为 Duration
		currentWindow = time.Duration(prevWindowHours) * time.Hour
	}

	// 2. 确定本次查询的 [lookbackStart, lookbackEnd] 窗口

	// A. lookbackEnd 确定：如果是首次查询，从当前时间开始；否则以上次的起点 (since) 作为本次的终点
	lookbackEnd := time.Now().UnixMilli()
	if since != 0 {
		lookbackEnd = since
	}

	// B. lookbackStart 确定：由 lookbackEnd 减去动态 currentWindow
	lookbackStart := lookbackEnd - int64(currentWindow/time.Millisecond)

	// C. 定义查询的绝对边界 (maxLookbackDays 天前)
	maxLookbackDuration := time.Duration(maxLookbackDays) * 24 * time.Hour
	absStartTime := time.Now().Add(-maxLookbackDuration).UnixMilli()

	// 3. 强制限制：确保 lookbackStart 不超过绝对边界
	isEnd := false
	if lookbackStart < absStartTime {
		lookbackStart = absStartTime
		isEnd = true // 达到历史尽头
	}

	// 4. 边界检查：如果窗口无效，直接返回空结果
	if lookbackStart >= lookbackEnd {
		return &types.UserFillOrderData{
			Data:            nil,
			Start:           lookbackStart,
			End:             lookbackEnd,
			HasMore:         false, // 无法继续追溯
			NextWindowHours: int(currentWindow / time.Hour),
		}, nil
	}

	// 5. 调用分批拉取逻辑
	orders, err := rest.UserFillOrdersByWindow(ctx, userAddress, lookbackStart, lookbackEnd)
	if err != nil {
		return nil, err
	}

	// 6. 根据结果，计算下一次建议的窗口大小 (核心自适应逻辑)
	nextWindowHours := int(currentWindow / time.Hour) // 默认保持不变

	if len(orders) == 0 {
		// **稀疏期：** 增大下次窗口 (指数级增长)
		newWindow := currentWindow * 2
		if newWindow > MaxPullWindow { // 假设 MaxPullWindow = 12 * time.Hour
			newWindow = MaxPullWindow
		}
		nextWindowHours = int(newWindow / time.Hour)

	} else if len(orders) > 1500 { // 阈值判断
		// **爆发期：** 缩小下次窗口
		newWindow := currentWindow / 2
		if newWindow < MinPullWindow { // 假设 MinPullWindow = 1 * time.Hour
			newWindow = MinPullWindow
		}
		nextWindowHours = int(newWindow / time.Hour)
	}

	// 7. 返回数据和**下次查询所需的状态**
	return &types.UserFillOrderData{
		Data:            orders,
		Start:           lookbackStart, // 下次查询的 since 参数
		End:             lookbackEnd,
		HasMore:         !isEnd,          // 基于 3. 的判断
		NextWindowHours: nextWindowHours, // 建议客户端下次查询传入的窗口大小
	}, nil
}

// 拉取 [since, now] 的所有增量数据，并在内部处理 now 的确定。
func (rest *HyperliquidRestClient) UserFillOrdersSinceTime(
	ctx context.Context,
	userAddress string,
	since int64, // 窗口起始时间戳 (毫秒)
) ([]*types.UserFillOrder, error) {

	now := time.Now().UnixMilli()
	if since >= now {
		return nil, nil
	}

	var allOrders []*types.UserFillOrder
	currentEnd := now // 从最新时间开始拉取
	loops := 0

	// 限制最大循环次数，防止极端情况下的无限循环
	const maxLoops = 50
	var requestCount = 0
	defer func() {
		log.Println("HyperliquidRestClient 本次UserFillOrdersSinceTime请求接口次数:", requestCount)
	}()
	for currentEnd > since && loops < maxLoops {
		requestCount += 1
		// 核心：确定当前查询的起始时间点
		// 我们从 currentEnd 往回推 MaxQueryChunk (3 小时) 作为默认的起始点
		startTimeCandidate := currentEnd - int64(MaxQueryChunk/time.Millisecond)

		// 确保起始点不会早于 'since' (上次计算的截止时间)
		if startTimeCandidate < since {
			startTimeCandidate = since
		}

		// 1. 发起 API 请求
		params := map[string]interface{}{
			"user": userAddress,
			// 查询范围：[startTimeCandidate, currentEnd)
			"startTime": startTimeCandidate,
			"endTime":   currentEnd,
		}

		var orders []*types.UserFillOrder
		if err := rest.doRequestWithContext(ctx, "/info", "userFillsByTime", params, &orders); err != nil {
			return nil, err
		}

		// 2. 检查返回结果
		if len(orders) == 0 {
			// **智能跳跃 (Jump over Empty Block):**
			// 如果这个时间块 [startTimeCandidate, currentEnd) 没有数据，
			// 那么我们可以安全地将下一个查询的结束时间点设置为 startTimeCandidate，
			// 从而跳过这 3 小时的空白期。
			currentEnd = startTimeCandidate
			loops++
			continue
		}

		// 3. 处理订单和确定下一次跳跃点
		// 假设 API 返回的订单是按时间降序排列 (最新到最旧)

		// 获取这批订单中最旧的订单时间
		oldestOrderTime := orders[len(orders)-1].Time

		// 过滤掉时间戳 <= 'since' 的订单
		newOrders := filterOrdersAfter(orders, since)
		allOrders = append(allOrders, newOrders...)

		// 4. 更新下一个查询的结束时间

		// **判断是否被截断：** 如果返回的数量达到了 API 限制
		if len(orders) == APIRecordLimit {
			// **应对截断 (Shrink Window):** // 知道数据被截断了，我们将下一个查询的结束时间设置为当前批次中最旧订单的时间。
			// 这样能保证下一批查询将接上当前批次。
			currentEnd = oldestOrderTime
		} else {
			// **安全跳跃 (Safe Jump):**
			// 如果返回数量没有达到上限，我们相信 [startTimeCandidate, currentEnd) 是完整的。
			// 下一个查询可以直接跳跃到 startTimeCandidate。
			currentEnd = startTimeCandidate
		}

		loops++
	}
	// 注意：由于我们是按时间块串行拉取，allOrders 需要按时间排序后再返回给上层计算逻辑
	sortAllOrders(allOrders)
	return allOrders, nil
}

// 在 [startTime, endTime] 时间窗口内，递归拉取所有交易记录，分批拉取，建议使用这个方法
func (rest *HyperliquidRestClient) UserFillOrdersByWindow(
	ctx context.Context,
	userAddress string,
	startTime int64, // 窗口起始时间戳 (毫秒)
	endTime int64,   // 窗口结束时间戳 (毫秒)
) ([]*types.UserFillOrder, error) {

	// 确保窗口有效
	if startTime >= endTime {
		return nil, nil
	}

	var allOrders []*types.UserFillOrder
	// 我们从 currentEnd 开始往回拉取
	pullEnd := endTime
	loops := 0

	// 限制最大循环次数，防止极端情况下的无限循环
	const maxLoops = 50
	var requestCount = 0
	defer func() {
		log.Printf("HyperliquidRestClient UserFillOrdersByWindow(%d-%d)请求接口次数: %d", startTime, endTime, requestCount)
	}()

	// 循环条件：拉取的结束时间 pullEnd 必须大于 startTime (窗口的起始时间)
	for pullEnd > startTime && loops < maxLoops {
		requestCount += 1
		loops++

		// 从窗口的绝对起始点拉取到 pullEnd
		startTimeCandidate := startTime

		// 1. 发起 API 请求
		params := map[string]interface{}{
			"user": userAddress,
			// 查询范围：[startTimeCandidate, pullEnd)
			"startTime": startTimeCandidate,
			"endTime":   pullEnd,
		}

		var orders []*types.UserFillOrder
		if err := rest.doRequestWithContext(ctx, "/info", "userFillsByTime", params, &orders); err != nil {
			return nil, err
		}

		// 2. 检查返回结果
		if len(orders) == 0 {
			// 如果拉取的 [startTimeCandidate, pullEnd) 范围内都没有数据
			// 且 startTimeCandidate == since，表示 [since, pullEnd) 都没有数据。
			if startTimeCandidate == startTime {
				// 整个 [since, pullEnd) 都没有数据，可以安全退出
				break
			}
			// 否则，让 pullEnd 缩小，进入下一次循环
			// ⚠️ 由于我们没有时间块限制，这里的逻辑应该更保守：将 pullEnd 设置为 startTimeCandidate
			// 但因为 startTimeCandidate 可能是 since，我们直接 break 即可。
			break
		}

		// 3. 处理订单和确定下一次跳跃点
		// 假设 API 返回的订单是按时间降序排列 (最新到最旧)

		// 过滤掉时间戳 <= 'since' 的订单（这是关键，只拉取 [since, pullEnd) 之间的）
		newOrders := filterOrdersAfter(orders, startTime) // 需要确保 filterOrdersAfter 过滤掉 <= since 的订单
		allOrders = append(allOrders, newOrders...)

		// 4. 更新下一个查询的结束时间

		// **应对截断：** 如果返回的数量达到了 API 限制
		if len(orders) >= APIRecordLimit {

			// 获取这批订单中最旧的订单时间
			oldestOrderTime := orders[len(orders)-1].Time

			// 将下一个查询的结束时间设置为 oldestOrderTime 减去 1 毫秒
			// 这样可以确保下一批查询 [startTime, pullEnd') 严格排除 oldestOrderTime 上的所有记录，避免边界死循环。
			newPullEnd := oldestOrderTime - 1

			// ⚠️ 安全检查：确保 newPullEnd 仍然大于 startTime。
			if newPullEnd <= startTime {
				// 如果减 1 毫秒后已经退到或退过 startTime，直接将 pullEnd 设置为 startTime，退出循环
				pullEnd = startTime
			} else {
				pullEnd = newPullEnd
			}
		} else {
			// **安全跳跃：**
			// 如果返回数量没有达到上限，我们相信 [startTimeCandidate, pullEnd) 是完整的。
			// 下一个查询可以直接跳跃到 startTimeCandidate (即 since)，然后循环条件会退出。
			pullEnd = startTimeCandidate
		}

		// 如果 pullEnd 已经 <= since，则循环退出
	}

	// 由于我们是按时间块串行拉取，allOrders 需要按时间排序后再返回给上层计算逻辑
	sortAllOrders(allOrders)
	return allOrders, nil
}

// 对返回的订单进行排序
func sortAllOrders(allOrders []*types.UserFillOrder) {
	if len(allOrders) == 0 {
		return
	}

	// sort.Slice 使用匿名函数进行排序，不需要实现完整的接口
	sort.Slice(allOrders, func(i, j int) bool {
		// 按照 Time 字段升序排列（从旧到新）
		if allOrders[i].Time == allOrders[j].Time {
			// 如果时间戳相同，使用 Tid 作为次要排序键（保证稳定性）
			return allOrders[i].Tid < allOrders[j].Tid
		}
		return allOrders[i].Time < allOrders[j].Time
	})
}

// 过滤掉时间戳小于或等于给定时间戳的订单。
// 确保只保留时间戳严格大于 'since' 的新订单。
func filterOrdersAfter(orders []*types.UserFillOrder, since int64) []*types.UserFillOrder {
	if len(orders) == 0 {
		return nil
	}

	var filteredOrders []*types.UserFillOrder
	// 遍历传入的订单切片
	for _, order := range orders {
		// 我们只关心时间戳严格大于 'since' 的订单
		if order.Time > since {
			filteredOrders = append(filteredOrders, order)
		}
		// 注意：如果 order.Time == startTime，则不包含，避免和上次计算的截止点重复。
	}

	// 注意：如果 API 已经保证订单是按时间排序的，我们可以进行优化：
	// 一旦遇到 order.Time <= since 的订单，就可以立即停止循环并返回，因为后面的订单时间只会更早或相等。
	// 但是为了鲁棒性，简单的遍历过滤是最安全的。

	return filteredOrders
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
	startTime := now.Add(-24 * 7 * time.Hour).UnixMilli()
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

func parsePortfolio(jsonData []byte) (*types.WhalePortfolio, error) {
	var raw [][]json.RawMessage
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		return nil, fmt.Errorf("解析顶层JSON失败: %v", err)
	}

	result := &types.WhalePortfolio{}

	for _, entry := range raw {
		if len(entry) != 2 {
			continue
		}

		// 第一个字段：周期名
		var period string
		if err := json.Unmarshal(entry[0], &period); err != nil {
			continue
		}

		// 第二个字段：详细数据
		var detail struct {
			AccountValueHistory [][]interface{} `json:"accountValueHistory"`
			PnlHistory          [][]interface{} `json:"pnlHistory"`
			Vlm                 string          `json:"vlm"`
		}
		if err := json.Unmarshal(entry[1], &detail); err != nil {
			continue
		}

		pd := types.PeriodData{}

		// 解析 AccountValueHistory
		for _, arr := range detail.AccountValueHistory {
			if len(arr) != 2 {
				continue
			}
			ts, ok1 := toInt64(arr[0])
			val, ok2 := toFloat64(arr[1])
			if ok1 && ok2 {
				pd.AccountValue = append(pd.AccountValue, types.DataPoint{
					Time:  time.UnixMilli(ts),
					Value: val,
				})
			}
		}

		// 解析 PnlHistory
		for _, arr := range detail.PnlHistory {
			if len(arr) != 2 {
				continue
			}
			ts, ok1 := toInt64(arr[0])
			val, ok2 := toFloat64(arr[1])
			if ok1 && ok2 {
				pd.Pnl = append(pd.Pnl, types.DataPoint{
					Time:  time.UnixMilli(ts),
					Value: val,
				})
			}
		}

		// 解析 vlm
		if f, ok := toFloat64(detail.Vlm); ok {
			pd.Vlm = f
		}

		// 分类存储到 WhalePortfolio
		switch period {
		case "day":
			result.Total.Day = pd
		case "week":
			result.Total.Week = pd
		case "month":
			result.Total.Month = pd
		case "allTime":
			result.Total.AllTime = pd
		case "perpDay":
			result.Perp.Day = pd
		case "perpWeek":
			result.Perp.Week = pd
		case "perpMonth":
			result.Perp.Month = pd
		case "perpAllTime":
			result.Perp.AllTime = pd
		default:
			// 忽略未知周期
		}
	}

	return result, nil
}

// ---------- 辅助函数 ----------

// interface{} → int64
func toInt64(v interface{}) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case string:
		var x int64
		_, err := fmt.Sscan(t, &x)
		return x, err == nil
	default:
		return 0, false
	}
}

// interface{} → float64
func toFloat64(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case string:
		var x float64
		_, err := fmt.Sscan(t, &x)
		return x, err == nil
	default:
		return 0, false
	}
}
