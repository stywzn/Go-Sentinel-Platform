package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	RabbitMQ RabbitMQConfig
}

type ServerConfig struct {
	Port string
}

type DatabaseConfig struct {
	Driver string
	Source string
}

type RabbitMQConfig struct {
	Url       string
	QueueName string
}

var GlobalConfig Config

func InitConfig() {
	viper.SetConfigName("config") // 配置文件名 (不带后缀)
	viper.SetConfigType("yaml")   // 文件格式
	viper.AddConfigPath(".")      // 搜索路径 (当前目录)

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	if err := viper.Unmarshal(&GlobalConfig); err != nil {
		log.Fatalf("Unable to decode into struct, %v", err)
	}
	log.Println("Configuration loaded successfully")
}
