package mq

import (
	"context"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQ struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
	Queue   string
}

func NewRabbitMQ(mqHost string, queueName string) *RabbitMQ {
	dsn := fmt.Sprintf("amqp://guest:guest@%s:5672/", mqHost)
	var conn *amqp.Connection
	var err error

	for i := 0; i < 5; i++ {
		conn, err = amqp.Dial(dsn)
		if err == nil {
			break
		}
		log.Printf("⚠️ 连接 MQ 失败，等待 2 秒重试... (%d/5)", i+1)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatalf("❌ 无法连接 RabbitMQ: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("❌ 无法创建 Channel: %v", err)
	}

	_, err = ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		log.Fatalf("❌ 无法声明队列: %v", err)
	}

	return &RabbitMQ{Conn: conn, Channel: ch, Queue: queueName}
}

// Publish 发送消息
func (r *RabbitMQ) Publish(ctx context.Context, body []byte) error {
	return r.Channel.PublishWithContext(ctx, "", r.Queue, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}

// 👇 新增：Consume 接收消息
// 返回一个只读的通道 (<-chan)，外面可以通过 range 来遍历消息
func (r *RabbitMQ) Consume() (<-chan amqp.Delivery, error) {
	msgs, err := r.Channel.Consume(
		r.Queue, // 队列名
		"",      // consumer tag
		true,    // auto-ack (自动确认收到，简单起见先设为 true)
		false,   // exclusive
		false,   // no-local
		false,   // no-wait
		nil,     // args
	)
	return msgs, err
}

func (r *RabbitMQ) Close() {
	r.Channel.Close()
	r.Conn.Close()
}
