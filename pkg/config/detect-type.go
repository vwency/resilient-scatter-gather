package config

import "os"

func DetectEnv() string {
	env := os.Getenv("APP_ENV")
	return env
}
