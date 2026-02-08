package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	// 👇 全部使用长名字引用，和 api-server 保持一致
	"github.com/stywzn/Go-Sentinel-Platform/internal/model"
	"github.com/stywzn/Go-Sentinel-Platform/pkg/config"
	"github.com/stywzn/Go-Sentinel-Platform/pkg/db"
	"github.com/stywzn/Go-Sentinel-Platform/pkg/mq"
)

func main() {
	// 1. 初始化
	config.InitConfig()
	db.Init() // <--- 必须是 Init()
	mq.Init()

	// 2. 开始消费
	msgs, err := mq.Channel.Consume(
		mq.QueueName, // queue
		"",           // consumer
		false,        // auto-ack (手动确认)
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		log.Fatal(err)
	}

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			// A. 解析 ID (因为 Server 发过来的是 ID 字符串)
			taskID, _ := strconv.Atoi(string(d.Body))
			log.Printf("Received a task: %d", taskID)

			// B. 查库改状态 -> Running
			var task model.Task
			// 注意：这里加了 .Error 检查，防止查不到报错
			if err := db.DB.First(&task, taskID).Error; err != nil {
				log.Printf("Task %d not found, skipping...", taskID)
				d.Ack(false) // 查不到也得确认，否则消息一直卡着
				continue
			}

			task.Status = "Running"
			db.DB.Save(&task)

			// C. 模拟干活 (5秒)
			log.Printf("Scanning target: %s ...", task.Target)
			scanResult := ScanTarget(task.Target)

			// D. 任务完成 -> Completed
			task.Status = "Completed"
			task.Result = scanResult
			db.DB.Save(&task)

			log.Printf("Task %d Done. Result: %s", taskID, scanResult)

			// E. 手动 ACK
			d.Ack(false)
		}
	}()

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
}

func ScanTarget(target string) string {
	ports := []string{"80", "443", "8080", "22", "3306"}
	var openPorts []string
	var wg sync.WaitGroup
	var mu sync.Mutex // 保护 openPorts 切片

	for _, port := range ports {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			address := fmt.Sprintf("%s:%s", target, p)
			// 尝试连接，超时设置为 2 秒
			conn, err := net.DialTimeout("tcp", address, 2*time.Second)
			if err == nil {
				conn.Close()
				mu.Lock()
				openPorts = append(openPorts, p)
				mu.Unlock()
			}
		}(port)
	}

	wg.Wait() // 等待所有端口扫完

	if len(openPorts) == 0 {
		return "No open ports found"
	}
	return fmt.Sprintf("Open Ports: %s", strings.Join(openPorts, ", "))
}
