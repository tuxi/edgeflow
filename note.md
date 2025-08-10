### 创建数据库
- 执行sql文件创建表
mysql -u root -p strategy_db < sql/init.sql

- 配置内网数据库docker访问权限
  在 UCloud 这种云服务器上，host.docker.internal 这个特殊域名是 macOS/Windows Docker Desktop 的功能，在 Linux 上是不存在的，所以会报`no such host`
  因为在 Linux 上，Docker 容器里想访问宿主机的 MySQL，需要换方法
  直接用宿主机内网 IP
  -- 1. 创建账号（如果不存在）
```shell
  CREATE USER IF NOT EXISTS 'root'@'172.17.%' IDENTIFIED BY 'root';
```

  -- 2. 授权
```shell
  GRANT ALL PRIVILEGES ON *.* TO 'root'@'172.17.%' WITH GRANT OPTION;
```

  -- 3. 刷新权限
```shell
  FLUSH PRIVILEGES;
```


### docker运行项目
- 编译项目
```shell
docker build -t edgeflow .
```

- 启动镜像
```shell
sudo docker run --restart=always -d --name edgeflow -e DB_HOST=host.docker.internal -e DB_PORT=3306 -e DB_USER=root -e DB_PASSWORD=root -e DB_NAME=strategy_db -test -p 12180:12180 -v /var/www/edgeflow/conf:/app/edgeflow/conf  edgeflow -test
```
sudo docker run --restart=always -d --name edgeflow -e DB_HOST=host.docker.internal -e DB_PORT=3306 -e DB_USER=root -e DB_PASSWORD=root -e DB_NAME=strategy_db -test -p 12180:12180 -v /Users/xiaoyuan/Desktop/work/LearningGolang/edgeflow/conf:/app/edgeflow/conf  edgeflow -test

