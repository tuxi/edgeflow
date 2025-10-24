## 架构升级与环境配置总结文档

### 项目目标

将原先所有资源密集型服务（Kafka, MySQL, Redis）从应用程序宿主机剥离，迁移至独立的腾讯云 CVM，以实现资源隔离、高稳定性和低运维负担。

### 最终架构概览

| 组件 | 部署位置 | 通信方式 | 核心功能 |
| :--- | :--- | :--- | :--- |
| **应用程序 (edgeflow)** | **旧服务器 (宿主机)** | 公网 $\leftrightarrow$ 公网 | 执行核心业务逻辑。 |
| **Kafka Broker** | **腾讯云 CVM** | 公网 TCP (9092) | 高速 K 线数据流传输。 |
| **MySQL DB** | **腾讯云 CVM** | 公网 TCP (3306) | 策略数据和历史记录存储。 |
| **Redis Server** | **腾讯云 CVM** | 公网 TCP (6379) | 高速缓存、最新状态存储。 |

---

## 腾讯云环境配置清单 (服务器端)

以下所有操作均在 **腾讯云 CVM (Ubuntu)** 服务器上执行。

### 1. 网络安全组配置 (外部防火墙)

**目标：** 限制入站访问，仅允许 **应用程序宿主机公网 IP (`A.B.C.D`)** 访问。

| 端口/服务 | 协议 | 来源 (Source) | 目的 |
| :--- | :--- | :--- | :--- |
| **SSH** | TCP | 22 | 您的管理 IP |
| **Kafka Broker** | TCP | **9092** | `A.B.C.D/32` |
| **MySQL DB** | TCP | **3306** | `A.B.C.D/32` |
| **Redis Server** | TCP | **6379** | `A.B.C.D/32` |

### 2. Kafka Broker 安装与配置 (KRAFT)

#### 核心配置 (`/opt/kafka/config/kraft/server.properties`)

```properties
process.roles=broker,controller
node.id=1
# 告知客户端连接此公网 IP（替换为实际 IP）
advertised.listeners=LISTENERS_PUBLIC://43.156.94.29:9092 
listeners=LISTENERS_PUBLIC://0.0.0.0:9092,CONTROLLER://127.0.0.1:9093
controller.quorum.voters=1@127.0.0.1:9093
inter.broker.listener.name=LISTENERS_PUBLIC

# 清理设置：不活跃 Consumer Group Offset 5小时后清除
offsets.retention.minutes=300
```


## 启动流程 (含数据清理)

- 权限修复： sudo chown -R <User>:<User> /opt/kafka (确保 service 用户有写入权限)

- 清理旧数据： sudo rm -rf /opt/kafka/data/*

- 格式化存储： kafka-storage.sh format -t "$(kafka-storage.sh random-uuid)" -c config/kraft/server.properties

启动服务： sudo systemctl start kafka

### 3. MySQL 数据库安装与配置
#### 安装与远程访问配置
```BASH
sudo apt install mysql-server -y
# 修改配置文件 /etc/mysql/mysql.conf.d/mysqld.cnf，设置 bind-address = 0.0.0.0
sudo systemctl restart mysql
```

#### 用户与授权配置

- 用户： edgeflow_user

- 授权： 允许所有 IP (%) 连接（安全性完全由腾讯云安全组保障）。
```mysql
-- 在 MySQL 命令行中执行：
CREATE USER 'edgeflow_user'@'%' IDENTIFIED BY 'YourSecurePassword';
GRANT ALL PRIVILEGES ON *.* TO 'edgeflow_user'@'%' WITH GRANT OPTION;
CREATE DATABASE strategy_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
FLUSH PRIVILEGES;
```

#### Binlog 清理策略
- 手动清理： PURGE BINARY LOGS BEFORE DATE_SUB(NOW(), INTERVAL N DAY);

- 自动清理（推荐）： 在 mysqld.cnf 中设置 expire_logs_days = 3。

### 4. Redis Server 安装与配置

- 安装与远程访问配置
```shell
sudo apt install redis-server -y
# 修改配置文件 /etc/redis/redis.conf
# 1. bind 0.0.0.0
# 2. requirepass YourSecureRedisPassword 
sudo systemctl restart redis-server
```

### 5.Docker 磁盘空间管理 (旧服务器)
#### 长期日志轮转配置（防止未来空间占满）
在 docker-compose.yml 中添加：
```dockerfile
services:
  your_service:
    logging:
      driver: "json-file"
      options:
        max-size: "100m"  # 每个日志文件最大 100MB
        max-file: "5"     # 最多保留 5 个日志文件
```

#### 一次性悬挂资源清理
docker system prune -f

# 应用程序宿主机配置清单
## 1.应用连接参数更新
> 更新 edgeflow 应用程序的配置文件或环境变量, 服务	新配置 (Remote)
- Kafka Broker	43.156.94.29:9092
- MySQL Host	43.156.94.29
- MySQL User/Pass	edgeflow_user/YourSecurePassword
- Redis Host	43.156.94.29:6379
- Redis Password	YourSecureRedisPassword

## 2. 隔离管理

- Topic 隔离： 开发/测试环境使用独立 Topic (market_prices_dev)。

- Group ID 隔离： 开发/测试环境使用独立 Group ID (aggregator_dev_yourname)。

## 3. 部署和验证

- 最终测试连接，重启应用程序，完成新架构的部署。