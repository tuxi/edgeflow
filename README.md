# EdgeFlow

EdgeFlow 是一个基于 TradingView 策略信号驱动的加密货币合约策略自动执行系统。该系统聚焦后端执行，借助 TradingView Webhook 信号，结合 Golang 服务完成自动下单和风控处理，目标是实现一个低成本、高可靠性的策略交易引擎。

---

## 🧠 核心理念

> **将 TradingView 的智能信号 + Golang 的稳定执行能力，结合成一个高效率的交易系统。**

---

## 🏗️ 系统架构概览

```
+--------------------+           +------------------------+
| TradingView 策略图表 |  ---webhook--->  | EdgeFlow 后端监听服务 |
+--------------------+           +------------------------+
                                             |
                                             | 解析信号 + 策略判断
                                             v
                              +----------------------------+
                              | 合约交易平台 API（如 OKX） |
                              +----------------------------+
```

---

## 📦 项目模块划分

* **TVWebhookReceiver**：监听 TradingView webhook 请求，接收 JSON 信号。
* **SignalParser**：解析 webhook 数据，提取交易指令（方向、标的、杠杆等）。
* **StrategyExecutor**：根据策略逻辑判断是否下单，并处理风控（止盈止损）。
* **ExchangeClient（OKX）**：封装交易所 API，负责发送下单、平仓等操作。
* **Logger / Notifier**：记录操作日志，必要时发送通知（如 Telegram、邮件等）。

---

## 🔧 技术栈

| 模块   | 技术                       | 说明               |
| ---- | ------------------------ | ---------------- |
| 接收器  | TradingView Webhook      | TV Pro 功能，用于发送信号 |
| 后端服务 | Golang + Gin/Fiber       | 轻量快速的 Web 服务框架   |
| 交易连接 | OKX REST API / Websocket | 合约交易支持、多空操作      |
| 数据存储 | SQLite / Redis（可选）       | 本地存储配置、状态缓存等     |
| 部署环境 | Mac 本地 / 云服务器            | 前期本地调试，后期上云运行    |

---

## 🚀 快速启动（本地）

```bash
git clone https://github.com/tuxi/edgeflow.git
cd edgeflow

# 安装依赖（如使用 Go modules）
go mod tidy

# 启动监听服务
go run main.go
```

---

## 🔐 安全建议

* Webhook 接口应设置 secret token 校验，防止被伪造请求。
* 交易 API 密钥需妥善保管，勿上传到公共仓库。
* 推荐使用服务器部署，并配合 nginx 做 HTTPS 和 IP 白名单控制。

---

## 🛣️ 后续计划（Roadmap）

* [ ] 多策略支持（区分策略ID）
* [ ] 可视化日志页面（用简单前端查看信号执行记录）
* [ ] Web Dashboard 管理界面（选做）
* [ ] 接入 AI 辅助判断（如止盈止损动态调整）

---

目标打造一个专业、可拓展的自动交易系统。
# edgeflow
