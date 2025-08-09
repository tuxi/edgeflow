### 创建数据库
- 执行sql文件创建表
mysql -u root -p strategy_db < sql/init.sql
### docker运行项目
- 编译项目
```shell
docker build -t edgeflow .
```

- 启动镜像
```shell
sudo docker run --restart=always -d --name edgeflow -test -p 12180:12180 -v /var/www/edgeflow/conf:/app/edgeflow/conf -v /var/www/edgeflow/deploy:/app/edgeflow/deploy -v /var/www/edgeflow/logs:/app/edgeflow/logs edgeflow -test
```