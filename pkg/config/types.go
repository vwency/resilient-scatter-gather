package config

import "time"

type ServiceConfig struct {
	App struct {
		Env         string `mapstructure:"env"`
		Port        string `mapstructure:"port"`
		LogLevel    string `mapstructure:"log_level"`
		ServiceName string `mapstructure:"service_name"`
	} `mapstructure:"app"`
	SLA struct {
		MaxResponseTimeMs int `mapstructure:"max_response_time_ms"`
		RequestTimeoutMs  int `mapstructure:"request_timeout_ms"`
	} `mapstructure:"sla"`
	Grpc struct {
		UserService        string `mapstructure:"user_service"`
		VectorService      string `mapstructure:"vector_service"`
		PermissionsService string `mapstructure:"permissions_service"`
		TimeoutMs          int    `mapstructure:"timeout_ms"`
	} `mapstructure:"grpc"`
	Degradation struct {
		UserTimeoutMs        int `mapstructure:"user_timeout_ms"`
		VectorTimeoutMs      int `mapstructure:"vector_timeout_ms"`
		PermissionsTimeoutMs int `mapstructure:"permissions_timeout_ms"`
	} `mapstructure:"degradation"`
}

func (c *ServiceConfig) GetSLATimeout() time.Duration {
	return time.Duration(c.SLA.MaxResponseTimeMs) * time.Millisecond
}

func (c *ServiceConfig) GetRequestTimeout() time.Duration {
	return time.Duration(c.SLA.RequestTimeoutMs) * time.Millisecond
}

func (c *ServiceConfig) GetGrpcTimeout() time.Duration {
	return time.Duration(c.Grpc.TimeoutMs) * time.Millisecond
}

func (c *ServiceConfig) GetUserDegradationTimeout() time.Duration {
	return time.Duration(c.Degradation.UserTimeoutMs) * time.Millisecond
}

func (c *ServiceConfig) GetVectorDegradationTimeout() time.Duration {
	return time.Duration(c.Degradation.VectorTimeoutMs) * time.Millisecond
}

func (c *ServiceConfig) GetPermissionsDegradationTimeout() time.Duration {
	return time.Duration(c.Degradation.PermissionsTimeoutMs) * time.Millisecond
}
