package main

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/stywzn/Go-Sentinel-Platform/internal/model"
	"github.com/stywzn/Go-Sentinel-Platform/pkg/config"
	"github.com/stywzn/Go-Sentinel-Platform/pkg/db"
	"github.com/stywzn/Go-Sentinel-Platform/pkg/mq"
)

type ScanRequest struct {
	Target string `json:"target" binding:"required"`
}

func main() {
	// 初始化各组件 (名字是 Init，不是 InitMySQL)
	config.InitConfig()
	db.Init()
	mq.Init()

	r := gin.Default()

	// 提交任务接口
	r.POST("/api/scan", func(c *gin.Context) {
		var req ScanRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 写入数据库 (状态: Pending)
		newTask := model.Task{
			Target: req.Target,
			Status: "Pending", // 保持状态大写或小写一致，建议 "Pending"
		}

		// 用 db.DB.Create
		if err := db.DB.Create(&newTask).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库写入失败"})
			return
		}

		// 发送消息到 RabbitMQ
		err := mq.Publish(strconv.Itoa(int(newTask.ID)))

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "任务入队失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "任务已提交",
			"task_id": newTask.ID,
		})
	})

	// 查询任务详情接口
	r.GET("/api/task", func(c *gin.Context) {
		id := c.Query("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "必须提供 id 参数"})
			return
		}

		var task model.Task
		// 根据 ID 查数据库
		if err := db.DB.First(&task, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"data": task,
		})
	})

	r.Run(":8080")
}
