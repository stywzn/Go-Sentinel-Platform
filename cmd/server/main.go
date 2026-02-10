package main

import (
	"log"
	"net"
	"os"

	"google.golang.org/grpc"

	pb "github.com/stywzn/Go-Cloud-Compute/api/proto"
	"github.com/stywzn/Go-Cloud-Compute/internal/server"
	"github.com/stywzn/Go-Cloud-Compute/pkg/db"
	"github.com/stywzn/Go-Cloud-Compute/pkg/mq"
)

func main() {
	// 1. 初始化数据库 (使用 pkg/db 包，不要自己在 main 里写连接代码)
	db.InitMySQL()

	// 自动迁移表结构 (使用全局的 db.DB)
	// 确保 AgentModel 和 JobRecord 在 internal/server 里定义了
	err := db.DB.AutoMigrate(&server.AgentModel{}, &server.JobRecord{})
	if err != nil {
		log.Printf("⚠️ 自动建表警告: %v", err)
	}

	// 2. 初始化 RabbitMQ
	mqHost := os.Getenv("MQ_HOST")
	if mqHost == "" {
		mqHost = "localhost"
	}
	rabbit := mq.NewRabbitMQ(mqHost, "job_queue")
	defer rabbit.Close()

	// 3. 准备 gRPC 服务
	// 注意：这里手动初始化 SentinelServer，把数据库传给它
	// 如果 server 包里有 NewSentinelServer 函数，最好用那个
	srv := &server.SentinelServer{
		DB: db.DB,
	}

	// 创建 gRPC 服务器
	grpcServer := grpc.NewServer()
	pb.RegisterSentinelServiceServer(grpcServer, srv)

	// 4. 准备 HTTP 服务
	// 关键点：参数顺序必须对应 (DB, gRPC服务, RabbitMQ)
	httpSrv := server.NewHttpServer(db.DB, srv, rabbit)

	srv.StartConsumer(rabbit)

	// 5. 启动监听
	lis, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatalf("❌ 端口监听失败: %v", err)
	}

	// 启动 HTTP (协程)
	go func() {
		log.Println("🚀 HTTP Server 启动在 :8080")
		httpSrv.Start()
	}()

	// 启动 gRPC (主线程阻塞)
	log.Println("🚀 Sentinel gRPC 启动在 :9090")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("❌ gRPC 服务崩溃: %v", err)
	}
}
