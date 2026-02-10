package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	pb "github.com/stywzn/Go-Cloud-Compute/api/proto"
	"github.com/stywzn/Go-Cloud-Compute/pkg/mq"
	"gorm.io/gorm"
)

// AgentModel 数据库表结构
type AgentModel struct {
	gorm.Model
	AgentID  string `gorm:"uniqueIndex;size:191"`
	Hostname string
	IP       string
	Status   string
}

// JobRecord 任务记录表结构
type JobRecord struct {
	gorm.Model
	JobID      string `gorm:"uniqueIndex;size:191"`
	AgentID    string `gorm:"index;size:191"`
	Type       string
	Result     string
	Payload    string
	Status     string
	ExecutedAt time.Time
}

// SentinelServer 主服务结构体
type SentinelServer struct {
	pb.UnimplementedSentinelServiceServer
	DB       *gorm.DB
	JobQueue sync.Map // Key: AgentID, Value: *pb.Job
}

// Register 注册接口
func (s *SentinelServer) Register(ctx context.Context, req *pb.RegisterReq) (*pb.RegisterResp, error) {
	// 这里假设 Hostname 就是 AgentID，或者你可以生成一个 UUID
	agentID := req.Hostname

	log.Printf("📝 [Register] 收到注册请求: %s (%s)", req.Hostname, req.Ip)

	var agent AgentModel
	result := s.DB.Where("agent_id = ?", agentID).First(&agent)

	if result.Error != nil {
		newAgent := AgentModel{
			AgentID:  agentID,
			Hostname: req.Hostname,
			IP:       req.Ip,
			Status:   "online",
		}
		s.DB.Create(&newAgent)
		log.Println("🆕 [DB] 新节点已入库")
	} else {
		agent.Status = "Online"
		agent.IP = req.Ip
		s.DB.Save(&agent)
		// log.Println("🔄 [DB] 节点信息已更新")
	}

	return &pb.RegisterResp{
		AgentId: agentID,
		Success: true,
	}, nil
}

// Heartbeat 流式心跳接口 (保留你的原版逻辑)
func (s *SentinelServer) Heartbeat(stream pb.SentinelService_HeartbeatServer) error {
	for {
		// 1. 接收心跳
		req, err := stream.Recv()
		if err == io.EOF {
			return nil // 客户端关闭连接
		}
		if err != nil {
			log.Printf("❌ 心跳接收错误: %v", err)
			return err
		}

		// 2. 检查是否有任务 (LoadAndDelete 取完即删，防止重复执行)
		if val, ok := s.JobQueue.LoadAndDelete(req.AgentId); ok {
			job := val.(*pb.Job)
			log.Printf("⚡ [Dispatch] 发现任务! 派发给 %s -> %s", req.AgentId, job.Payload)

			// 发送任务给 Agent
			err := stream.Send(&pb.HeartbeatResp{
				Job: job,
			})
			if err != nil {
				log.Printf("❌ 发送任务失败: %v", err)
				return err
			}
		} else {
			// 没有任务，发送空响应维持心跳
			stream.Send(&pb.HeartbeatResp{ConfigOutdated: false})
		}
	}
}

// ReportJobStatus 任务结果上报
func (s *SentinelServer) ReportJobStatus(ctx context.Context, req *pb.ReportJobReq) (*pb.ReportJobResp, error) {
	log.Printf("✅ [Report] 任务汇报! Agent: %s | Job: %s | 结果: %s",
		req.AgentId, req.JobId, req.Result)

	record := JobRecord{
		JobID:      req.JobId,
		AgentID:    req.AgentId,
		Type:       "SHELL", // 记录为 SHELL
		Payload:    "Unknown",
		Result:     req.Result,
		Status:     req.Status,
		ExecutedAt: time.Now(),
	}

	if err := s.DB.Create(&record).Error; err != nil {
		log.Printf("❌ [DB] 保存任务记录失败: %v", err)
	}

	return &pb.ReportJobResp{Received: true}, nil
}

// StartConsumer 启动 RabbitMQ 消费者
func (s *SentinelServer) StartConsumer(rabbit *mq.RabbitMQ) {
	msgs, err := rabbit.Consume()
	if err != nil {
		log.Printf("❌ 无法启动消费者: %v", err)
		return
	}

	log.Println("🎧 MQ 消费者已启动，正在等待任务...")

	go func() {
		for d := range msgs {
			log.Printf("📥 [消费者] 收到 MQ 消息: %s", d.Body)

			var jobReq struct {
				Target string `json:"target"`
				Cmd    string `json:"cmd"`
			}

			if err := json.Unmarshal(d.Body, &jobReq); err != nil {
				log.Printf("❌ 解析消息失败: %v", err)
				continue
			}

			// 构造 Proto 对象
			jobID := fmt.Sprintf("mq-%d", time.Now().Unix())
			job := &pb.Job{
				JobId: jobID,
				// 👇👇👇 关键修改：你的 Proto 里只有 SHELL，没有 EXEC，必须改！ 👇👇👇
				Type:    pb.JobType_SHELL,
				Payload: jobReq.Cmd,
			}

			// 存入 Map，等待 Heartbeat 来取
			s.JobQueue.Store(jobReq.Target, job)
			log.Printf("✅ 任务已由 MQ 转入内存队列 -> Agent: %s", jobReq.Target)
		}
	}()
}
