FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download

COPY . .

# 编译两个二进制文件
# CGO_ENABLED=0 表示静态编译，不需要依赖系统库
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o agent ./cmd/agent/main.go

# ----------------------------------------------------

# 运行 (Runner)
# 使用极小的 Alpine 镜像 (只有 5MB)
FROM alpine:latest

RUN apk --no-cache add iputils curl

WORKDIR /root/

# 从构建阶段把编译好的文件拿过来
COPY --from=builder /app/server .
COPY --from=builder /app/agent .

# 默认运行 api-server (可以在 docker-compose 里覆盖)
CMD ["./server"]