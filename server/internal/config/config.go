package config

import "os"

type Config struct {
	Addr        string
	JWTSecret   string
	DatabaseDSN string
	RedisAddr   string
	RedisDB     int
	CORSOrigins string
}

func Load() Config {
	c := Config{Addr: ":8080", JWTSecret: "stockflow-development-secret", DatabaseDSN: "root:stockflow@tcp(mysql:3306)/stockflow?charset=utf8mb4&parseTime=True&loc=Local", RedisAddr: "redis:6379", CORSOrigins: "http://localhost:4310,http://127.0.0.1:4310,http://localhost:4330,http://127.0.0.1:4330"}
	if value := os.Getenv("APP_ADDR"); value != "" {
		c.Addr = value
	}
	if value := os.Getenv("JWT_SECRET"); value != "" {
		c.JWTSecret = value
	}
	if value := os.Getenv("MYSQL_DSN"); value != "" {
		c.DatabaseDSN = value
	}
	if value := os.Getenv("REDIS_ADDR"); value != "" {
		c.RedisAddr = value
	}
	if value := os.Getenv("CORS_ORIGINS"); value != "" {
		c.CORSOrigins = value
	}
	return c
}
