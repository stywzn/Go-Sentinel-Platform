package server

import (
	"encoding/json"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/stywzn/Go-Cloud-Compute/pkg/mq"
	"gorm.io/gorm"
)

type HttpServer struct {
	DB  *gorm.DB
	Srv *SentinelServer
	MQ  *mq.RabbitMQ
}

func NewHttpServer(db *gorm.DB, srv *SentinelServer, rabbit *mq.RabbitMQ) *HttpServer {
	return &HttpServer{
		DB:  db,
		Srv: srv,
		MQ:  rabbit, // 现在这里认识 rabbit 了
	}
}

type JobRequest struct {
	TargetAgent string `json:"target"`
	Cmd         string `json:"cmd"`
}

func (h *HttpServer) Start() {
	r := gin.Default()

	r.GET("/agent", func(c *gin.Context) {
		var agents []AgentModel
		// 确保这里不出错，如果你没有定义 AgentModel，可能需要检查一下
		h.DB.Find(&agents)
		c.JSON(200, gin.H{"code": 200, "data": agents})
	})

	r.POST("/job", func(c *gin.Context) {
		var req JobRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "JSON 格式不对"})
			return
		}

		// 1. 转成 JSON 字节
		body, _ := json.Marshal(req)
		log.Printf("[HTTP] 管理员下发任务 -> %s : %s", req.TargetAgent, req.Cmd)

		// 2. 发送到 MQ (现在参数类型匹配了)
		err := h.MQ.Publish(c.Request.Context(), body)

		if err != nil {
			log.Printf("MQ 发送失败: %v", err)
			c.JSON(500, gin.H{"error": "任务入队失败"})
			return
		}

		c.JSON(200, gin.H{
			"code": 200,
			"msg":  "任务已发送到消息队列 (异步处理)",
			"info": req,
		})
	})

	if err := r.Run(":8080"); err != nil {
		log.Fatal("HTTP Server 启动失败: ", err)
	}
}
