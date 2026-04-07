package config

type ServerConfig struct {
	Port      int     `json:"port"`
	LatencyMS int     `json:"latency_ms"`
	Capacity  int     `json:"capacity"`
	ErrorRate int     `json:"error_rate"`
	Weight    int     `json:"weight"`
	IsCanary  bool    `json:"is_canary"`
}

type StrategyConfig struct {
	Strategy       string  `json:"strategy"`
	CanaryPort     int     `json:"canary_port"`
	CanaryPercent  int     `json:"canary_percent"`
	ErrorThreshold float64 `json:"error_threshold"`
}