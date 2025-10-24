# Kafka 安装与 Docker 容器访问指南

本文档总结了在 Ubuntu 上安装 Kafka、配置 KRaft 模式、处理下载中断、以及 Docker 容器访问 Kafka 的完整流程。

# 按照java
```shell
 apt update
 apt install -y openjdk-17-jdk
```

# 1. Kafka 下载
   由于新加坡节点访问 Apache 官方源可能很慢，可使用断点续传工具 aria2c。
   bash
   apt  install aria2
   cd /opt
   aria2c -x 16 -s 8 https://archive.apache.org/dist/kafka/3.7.0/kafka_2.13-3.7.0.tgz
# -x 16：最多 16 个连接
# -s 8：分 8 个线程下载

# 如果下载中途断开，可直接重复命令，aria2 会续传
本地下载后上传服务器：
bash
scp kafka_2.13-3.7.0.tgz ubuntu@your_server_ip:/home/ubuntu/

# 上传后在服务器上确认文件大小
ls -lh kafka_2.13-3.7.0.tgz
## 2. Kafka 安装

# 解压 Kafka 文件
cd /opt
tar -xzf kafka_2.13-3.7.0.tgz
mv kafka_2.13-3.7.0 kafka


# 创建数据目录
mkdir -p /opt/kafka/data
## 3. Kafka 配置（单节点 KRaft 模式，Docker 可访问宿主机）

# 编辑 /opt/kafka/config/kraft/server.properties
vim /opt/kafka/config/kraft/server.properties
text
# ------------------
# Broker & Controller 配置
# ------------------
process.roles=broker,controller
node.id=1
controller.quorum.voters=1@172.17.0.1:9093
log.dirs=/opt/kafka/data

# ------------------
# Listener 配置
# ------------------
listeners=PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:9093
advertised.listeners=PLAINTEXT://172.17.0.1:9092
controller.listener.names=CONTROLLER
listener.security.protocol.map=PLAINTEXT:PLAINTEXT,CONTROLLER:PLAINTEXT
inter.broker.listener.name=PLAINTEXT

# 注释必须独占一行，不要跟在配置行后面
## 4. 启动 Kafka

# 初始化kafka
bin/kafka-storage.sh format   --config config/kraft/server.properties   --cluster-id $(bin/kafka-storage.sh random-uuid)

# 手动启动
cd /opt/kafka
bin/kafka-server-start.sh -daemon config/kraft/server.properties

# 查看日志确认启动成功
tail -f logs/server.log

# 等待日志出现 Controller quorum initialized successfully
# INFO 日志如 Log loaded for partition ... 属于正常信息
# 停止 Kafka
bin/kafka-server-stop.sh

# 创建topic
bin/kafka-topics.sh --bootstrap-server 172.17.0.1:9092   --create --topic marketdata_subscribe   --partitions 1   --replication-factor 1

# 如果提示 No kafka server to stop，说明 Kafka 当前没有运行，可直接启动
## 5. 使用 systemd 管理 Kafka（推荐）

# 创建 systemd 服务文件 /etc/systemd/system/kafka.service
vim /etc/systemd/system/kafka.service
ini
[Unit]
Description=Apache Kafka Server
After=network.target

[Service]
Type=simple
User=root
Group=root
ExecStart=/opt/kafka/bin/kafka-server-start.sh /opt/kafka/config/kraft/server.properties
ExecStop=/opt/kafka/bin/kafka-server-stop.sh
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
# 管理命令
sudo systemctl daemon-reload
sudo systemctl start kafka
sudo systemctl status kafka
sudo systemctl restart kafka
sudo systemctl enable kafka  # 开机自启
journalctl -u kafka -f      # 查看日志
## 6. Docker 容器访问 Kafka

# 默认 Docker 网桥 IP 为 172.17.0.1
ip addr show docker0

# 容器中访问 Kafka
kafHost := os.Getenv("KAFKA_HOST")
kafPort := os.Getenv("KAFKA_PORT")
kafBroker := fmt.Sprintf("%s:%s", kafHost, kafPort)
if kafHost == "" || kafPort == "" {
kafBroker = "172.17.0.1:9092"  # 默认宿主机 IP
}

# 测试容器能否访问宿主机 Kafka
docker exec -it <container_name> ping -c 3 172.17.0.1
docker exec -it <container_name> kafka-console-producer.sh --broker-list 172.17.0.1:9092 --topic test
## 7. 常见坑与注意事项

# 1. 下载中断
# - 使用 aria2c 支持断点续传
# - 本地下载后上传服务器

# 2. server.properties 配置错误
# - advertised.listeners 后不能带注释
# - controller.listener.names 必须出现在 listeners 中

# 3. Kafka 启动日志
# - INFO 日志如 Log loaded for partition ... 正常
# - ERROR/FATAL 日志才需要关注

# 4. 容器访问 Kafka
# - 确保宿主机 IP 可达
# - listeners 必须监听 0.0.0.0
# - Docker 容器不必带环境变量，只要 advertised.listeners 配置正确
## 8. 总结

# Kafka 下载 → 上传/断点续传 → 解压 → 配置 KRaft → 启动 → Docker 容器访问
# 推荐用 systemd 管理 Kafka，修改配置后直接 systemctl restart kafka
# 配置注意点：listeners、advertised.listeners、controller.listener.names
# 日志 INFO 很多，正常，只关注 ERROR/FATAL