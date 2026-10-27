package config

import "os"

type Config struct {
	HTTPAddr    string
	CORSOrigins string
}

func Load() Config {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	origins := os.Getenv("CORS_ORIGINS")
	if origins == "" {
		origins = "http://localhost:3000"
	}
	return Config{HTTPAddr: addr, CORSOrigins: origins}
}
