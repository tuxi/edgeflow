# 📈 Real-Time Market Data Gateway (实时市场数据网关)

## 🎯 本次架构优化：痛点与目标

| 维度 | 优化前痛点 (Old Pain Point) | 优化后目标 (New Goal) |
| :--- | :--- | :--- |
| **耦合度** | 数据处理 (OKXCandleService) 与数据推送 (Gateway) 紧密耦合。 | **解耦**：引入 Kafka 作为数据总线，实现生产者和消费者服务的完全隔离。 |
| **可扩展性**| Gateway 性能受限于 CandleService 的处理能力。| **水平扩展**：Gateway 和 CandleService 可独立扩容，支持更高的并发和吞吐量。 |
| **可靠性** | 实时数据流经内存，服务重启可能丢失瞬时更新。 | **数据持久化**：利用 Kafka 的持久化存储特性，提供更可靠的数据缓冲。 |
| **数据格式**| 消息传输格式不统一，效率不高。| **Protobuf 标准化**：统一使用 Protobuf 序列化，提高传输效率和性能。 |

---

## 🛠 本地开发环境与依赖

本项目基于 macOS (Homebrew) 进行开发，需要以下环境依赖：

### 1. 核心依赖安装

| 依赖 | 安装命令 | 作用 |
| :--- | :--- | :--- |
| **Go Lang** | `brew install go` | 主编程语言。 |
| **Java JRE** | `brew install openjdk` | Kafka 服务的运行环境。 |
| **Protobuf Compiler** | `brew install protobuf` | 用于编译 `.proto` 文件。 |
| **Go Protobuf 插件**| `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest` <br> `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest` | 负责将 Protobuf 定义生成 Go 语言模型代码。 |

### 2. Kafka 服务端安装与配置

本项目使用 **Homebrew** 安装和管理 Kafka，地址默认为 `localhost:9092`。

#### A. 安装与启动

```bash
# 1. 安装 Kafka (包含依赖 Zookeeper)
brew install kafka

# 2. 启动 Kafka 服务
brew services start kafka
```

#### B. 配置与 Topic 自动创建 (可选)

为方便开发，建议开启 Topic 自动创建。

停止服务： brew services stop kafka

编辑配置文件： nano /usr/local/etc/kafka/server.properties

确认/添加以下配置：
```shell
# 启用 Topic 自动创建，方便开发
auto.create.topics.enable=true 

# 确保监听地址正确
listeners=PLAINTEXT://:9092
advertised.listeners=PLAINTEXT://localhost:9092
```

重启服务： `brew services start kafka`

#### 3. Kafka Topic 管理
本项目使用了三个 Topic 进行数据隔离。即使开启了自动创建，仍推荐手动创建以指定分区数

编辑profile文件
```shell
vim ~/.bash_profile
```
最后一行添加 Broker 地址环境变量`export KAFKA_BOOTSTRAP_SERVER="localhost:9092"`

让新的配置生效
```shell
source ~/.base_profile
```

手动创建 Ticker Topic，也可以让项目中自动创建
```shell
# 创建 Ticker Topic (分区数高，提高吞吐量)
kafka-topics --create --topic marketdata_ticker --bootstrap-server $KAFKA_BOOTSTRAP_SERVER --partitions 3 --replication-factor 1

# 创建 Subscribe Topic (分区数高，处理订阅推送)
kafka-topics --create --topic marketdata_subscribe --bootstrap-server $KAFKA_BOOTSTRAP_SERVER --partitions 8 --replication-factor 1

# 创建 System Topic (分区数低，用于低频系统状态)
kafka-topics --create --topic marketdata_system --bootstrap-server $KAFKA_BOOTSTRAP_SERVER --partitions 1 --replication-factor 1

# 验证所有 Topic
kafka-topics --list --bootstrap-server $KAFKA_BOOTSTRAP_SERVER
```

## 💻 Protobuf 模型生成步骤
当数据模型 (.proto 文件) 发生变化时，需要重新生成 Go 语言的结构体文件
```shell
cd pkg/protobuf
protoc --go_out=. --go_opt=paths=source_relative market_data.proto
```

### 核心 Go 代码实现 (internal/pkg/kafka)
#### 1. 生产者 (Producer)

核心逻辑集中在 kafkaProducer.Produce() 的 switch 路由，确保不同 Topic 的消息被导向到对应 Topic 的 kafka.Writer，并处理 Protobuf 序列化。

文件： pkg/kafka/kafka_producer.go

注意： 确保 NewKafkaProducer 初始化了 systemWriter，并在 Produce 方法中加入了 case "marketdata_system": 逻辑。

#### 2. 消费者 (Consumer)

核心逻辑是创建一个 kafka.Reader，并在 Goroutine 中循环调用 r.FetchMessage() 或 r.ReadMessage() 持续获取数据，并通过 Go Channel 返回给上层应用。

文件： pkg/kafka/kafka_consumer.go

> 注意： 消费者必须设置 GroupID，以实现消费者组（Consumer Group）的负载均衡。

## 🎯 下一阶段工作
> 当前阶段，数据流：OKXCandleService (Protobuf) → Kafka Broker (已验证)。

下一阶段需完成 SubscriptionGateway 的消费者集成和最终的 WebSocket 推送。

- 1.Gateway Consumer 逻辑：

  * 在 SubscriptionGateway 中启动 Kafka 消费者协程。

  * 从 Kafka Channel 中读取消息 (kafka.Message)。

  * 进行 Protobuf 反序列化，还原为 Go 结构体。

- 2.WebSocket 推送：

  * 实现 WebSocket 握手和连接管理。

  * 解析客户端的 SUBSCRIBE 指令，记录订阅信息。

  * 将 Kafka 接收到的实时更新 序列化为 JSON（推荐）或 Protobuf 二进制格式，通过 WebSocket 推送给对应已订阅的客户端。

docker run --restart=always -d --net strategy-net --name edgeflow -e KAFKA_HOST=10.35.76.97 -e KAFKA_PORT=9092 -e REDIS_HOST=172.17.0.1 -e REDIS_PORT=6379 -e DB_HOST=172.17.0.1 -e DB_PORT=3306 -e DB_USER=root -e DB_PASSWORD=root -e DB_NAME=strategy_db -test -p 12180:12180 -v /var/www/edgeflow/conf:/app/edgeflow/conf  edgeflow -test