package config

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/viper"
)

func Init(env, servicePath string, cfg any) {
	if env == "" {
		env = os.Getenv("ENV")
	}

	if env == "" {
		viper.SetConfigName("config")
	} else {
		viper.SetConfigName("config." + env)
	}

	viper.SetConfigType("yaml")

	viper.AddConfigPath(fmt.Sprintf("./config/%s", servicePath))
	viper.AddConfigPath(fmt.Sprintf("/app/config/%s", servicePath))
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	viper.AutomaticEnv()

	if err := viper.Unmarshal(cfg); err != nil {
		log.Fatalf("Unable to decode into struct: %v", err)
	}

	fmt.Printf("[CONFIG] Loaded config: %s\n", viper.ConfigFileUsed())
}
