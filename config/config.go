package config

type ServerConfig struct {
	Port      int `json:"port"`
	LatencyMS int `json:"latency_ms"`
	Capacity  int `json:"capacity"`
	ErrorRate int `json:"error_rate"`
	Weight    int `json:"weight"`
}

type StrategyConfig struct {
	Strategy string `json:"strategy"`
}