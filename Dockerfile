# FROM 表示设置要制作的镜像基于哪个镜像，FROM指令必须是整个Dockerfile的第一个指令，如果指定的镜像不存在默认会自动从Docker Hub上下载。
FROM golang:1.24.0-alpine  AS builder
ENV GO111MODULE=on
#ENV GOPROXY=https://goproxy.io,direct
#安装编译需要的环境gcc等
RUN apk add build-base git
# 安装Swagger
RUN go install github.com/swaggo/swag/cmd/swag@latest

# 指定工作目录（或者称为当前目录），以后各层的当前目录就被改为指定的目录，如果目录不存在，WORKDIR 会帮你建立目录
WORKDIR /code
# COPY 指令将从构建上下文目录中 <源路径> 的文件/目录复制到新的一层的镜像内的 <目标路径> 位置
COPY . /code/src
WORKDIR /code/src
RUN swag init -g cmd/main.go
RUN make linux  

#编译
FROM alpine
RUN apk --no-cache add tzdata ca-certificates libc6-compat libgcc libstdc++
RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && echo "Asia/Shanghai" > /etc/timezone 
COPY --from=builder  /code/src/dist/linux_amd64/edgeflow  /app/edgeflow/edgeflow
#COPY --from=builder  /code/src/dict  /app/edgeflow/dict
COPY --from=builder  /code/src/docs  /app/edgeflow/docs

WORKDIR /app/edgeflow

# 容器对外暴露的端口号，这里和配置文件保持一致就可以
EXPOSE 12180

VOLUME ["/app/edgeflow/conf","/app/edgeflow/logs","/app/edgeflow/files/avatar","/app/edgeflow/files/upload"]

ENTRYPOINT  ["/app/edgeflow/edgeflow"]
