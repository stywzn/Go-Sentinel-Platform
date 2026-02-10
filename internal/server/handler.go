package server

import (
	"context"
	"log"
	"time"

	pb "github.com/stywzn/Go-Cloud-Compute/api/proto"
	"gorm.io/gorm"
)

type AgentModel struct {
	gorm.Model
	AgentID  string `gorm:"uniqueIndex;size:191"`
	Hostname string
	IP       string
	Status   string
}

type SentinelServer struct {
	pb.UnimplementedSentinelServiceServer
	DB *gorm.DB
}

func (s *SentinelServer) Register(ctx context.Context, req *pb.RegisterReq) (*pb.RegisterResp, error) {
	agentID := req.Hostname

	log.Printf(" [Register] 收到注册请求: %s (%s)", req.Hostname, req.Ip)

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
		log.Println(" [DB] 新节点已入库")
	} else {
		agent.Status = "Online"
		agent.IP = req.Ip
		s.DB.Save(&agent)
		log.Println(" [DB] 节点信息已更新")
	}

	return &pb.RegisterResp{
		AgentId: agentID,
		Success: true,
	}, nil
}

func (s *SentinelServer) Heartbeat(stream pb.SentinelService_HeartbeatServer) error {
	for {

		req, err := stream.Recv()

		if err != nil {
			log.Printf(" 接收错误: %v", err)
			return err
		}

		log.Printf(" [Heartbeat] 来自: %s", req.AgentId)

		if time.Now().Unix()%10 == 0 {
			log.Printf(" [Dispatch] 正在派发 PING 任务给 %s", req.AgentId)
			err := stream.Send(&pb.HeartbeatResp{
				Job: &pb.Job{
					JobId:   "job-" + req.AgentId,
					Type:    pb.JobType_PING,
					Payload: "8.8.8.8",
				},
			})
			if err != nil {
				return err
			}
		} else {
			stream.Send(&pb.HeartbeatResp{ConfigOutdated: false})
		}
	}
}

func (s *SentinelServer) ReportJobStatus(ctx context.Context, req *pb.ReportJobReq) (*pb.ReportJobResp, error) {

	log.Printf(" [Report] 收到任务汇报! Agent: %s | Job: %s | 状态: %s | 结果: %s",
		req.AgentId, req.JobId, req.Status, req.Result)
	return &pb.ReportJobResp{Received: true}, nil
}
