package okx

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// okx的开发接口，不需要apikey

// OkxClient 结构体封装了与 OKX 公开 REST API 通信所需的一切
type PublicClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewOkxClient 初始化并返回一个新的 OkxClient 实例
func NewPublicClient() *PublicClient {
	return &PublicClient{
		// OKX V5 基础公共 API 地址
		baseURL: "https://www.okx.com/api/v5",
		// 使用自定义的 HTTP Client，设置合理的超时时间
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

const maxRetries = 3

// GetInstrumentsWithRetry 封装了 GetInstruments，并添加了重试逻辑
func (c *PublicClient) GetInstrumentsWithRetry(ctx context.Context, instType string) ([]InstrumentRaw, error) {
	var instruments []InstrumentRaw
	var err error

	// 初始化退避时间
	backoffTime := 10 * time.Second

	for i := 0; i < maxRetries; i++ {
		instruments, err = c.GetInstruments(ctx, instType)

		// 成功获取数据
		if err == nil {
			return instruments, nil
		}

		// 失败，记录日志并准备重试
		log.Printf("WARN: Attempt %d failed to get instruments for %s: %v. Retrying in %v...",
			i+1, instType, err, backoffTime)

		// 如果是最后一次尝试，则不再等待
		if i == maxRetries-1 {
			break
		}

		// 等待退避时间
		select {
		case <-ctx.Done():
			// 上下文被取消，停止重试
			return nil, ctx.Err()
		case <-time.After(backoffTime):
			// 继续下一次循环
		}

		// 指数退避：下次等待时间翻倍 (或使用 3倍等)
		backoffTime *= 3
	}

	// 达到最大重试次数后仍然失败
	return nil, fmt.Errorf("在 %d 次尝试后，无法获取 %s 交易对列表，最终错误: %w", maxRetries, instType, err)
}

// GetInstruments 获取所有交易产品列表
// instType: SPOT, SWAP, FUTURES 等
func (c *PublicClient) GetInstruments(ctx context.Context, instType string) ([]InstrumentRaw, error) {

	// 拼接 API 端点和参数
	endpoint := fmt.Sprintf("/public/instruments?instType=%s", instType)

	var instruments []InstrumentRaw
	if err := c.doPublicGet(ctx, endpoint, &instruments); err != nil {
		return nil, fmt.Errorf("获取 %s 交易对失败: %w", instType, err)
	}

	return instruments, nil
}

// doPublicGet 执行通用的 GET 请求，处理 JSON 解析和错误
func (c *PublicClient) doPublicGet(ctx context.Context, endpoint string, result interface{}) error {
	url := fmt.Sprintf("%s%s", c.baseURL, endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置一些必要的头信息（可选，但推荐）
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 返回非 200 状态码错误
		return fmt.Errorf("API返回非成功状态码: %d", resp.StatusCode)
	}

	// OKX API 的标准 JSON 格式：{"code":"0", "msg":"", "data":[...]}
	var apiResponse struct {
		Code string          `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"` // 使用 RawMessage 接收实际的数据数组
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return fmt.Errorf("解析API响应JSON失败: %w", err)
	}

	if apiResponse.Code != "0" {
		// OKX API 业务错误（code!=0）
		return fmt.Errorf("OKX API错误, Code: %s, Msg: %s", apiResponse.Code, apiResponse.Msg)
	}

	// 将 Data 字段（通常是一个 JSON 数组）解析到目标结构体
	if err := json.Unmarshal(apiResponse.Data, result); err != nil {
		return fmt.Errorf("解析Data字段到目标结构体失败: %w", err)
	}

	return nil
}

// OkxInstrumentRaw 对应 OKX API 返回的单个交易对信息
type InstrumentRaw struct {
	// 基础信息
	InstId   string `json:"instId"`   // 交易对 ID (如 ETH-AUD)
	InstType string `json:"instType"` // 交易对类型 (SPOT/SWAP/FUTURES)
	BaseCcy  string `json:"baseCcy"`  // 主币代码 (如 ETH)
	QuoteCcy string `json:"quoteCcy"` // 计价币代码 (如 AUD)
	State    string `json:"state"`    // 交易状态 (如 live)
	ListTime string `json:"listTime"` // 上线时间戳 (毫秒)

	// 精度/步长信息
	TickSz string `json:"tickSz"` // 价格精度/步长 (Price Precision)
	MinSz  string `json:"minSz"`  // 最小交易数量 (Quantity Precision)
	LotSz  string `json:"lotSz"`  // 合约乘数/最小下单量 (Quantity Precision - 另一种形式，这里用 minSz)

	// 其他不常用或与合约相关的字段 (仅用于接收，不一定存储)
	Category string `json:"category"`
}
