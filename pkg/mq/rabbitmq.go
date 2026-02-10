package mq

import (
	"fmt"
	"log"

	"github.com/streadway/amqp"
)

type RabbitMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	Queue   string
}

func NewRabbitMQ(mqHost string, queueName string) *RabbitMQ {
	// 连接格式: amqp://账号:密码@地址:端口/
	dsn := fmt.Sprintf("amqp://guest:guest@%s:5672/", mqHost)
	conn, err := amqp.Dial(dsn)
	if err != nil {
		log.Fatalf("❌ 无法连接 RabbitMQ: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("❌ 无法创建 Channel: %v", err)
	}

	// 声明队列 (如果没有就创建)
	_, err = ch.QueueDeclare(
		queueName, // 队列名字
		true,      // 持久化 (重启还在)
		false,     // 自动删除
		false,     // 排他性
		false,     // NoWait
		nil,       // 参数
	)
	if err != nil {
		log.Fatalf("❌ 无法声明队列: %v", err)
	}

	return &RabbitMQ{
		conn:    conn,
		channel: ch,
		Queue:   queueName,
	}
}

// Publish 发送消息 (生产者)
func (r *RabbitMQ) Publish(body string) error {
	err := r.channel.Publish(
		"",      // Exchange
		r.Queue, // Routing Key (队列名)
		false,   // Mandatory
		false,   // Immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		},
	)
	return err
}

// Consume 接收消息 (消费者) - 返回一个只读通道
func (r *RabbitMQ) Consume() (<-chan amqp.Delivery, error) {
	msgs, err := r.channel.Consume(
		r.Queue, // 队列名
		"",      // Consumer Tag
		true,    // Auto Ack (自动确认收到)
		false,   // Exclusive
		false,   // No Local
		false,   // No Wait
		nil,     // Args
	)
	return msgs, err
}

func (r *RabbitMQ) Close() {
	r.channel.Close()
	r.conn.Close()
}
