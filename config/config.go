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

type RateLimitConfig struct {
    Algorithm         string `json:"algorithm"`
    RequestsPerSecond int    `json:"requests_per_second"`
    BucketSize        int    `json:"bucket_size"`
    MaxConcurrent     int    `json:"max_concurrent"`
    WindowSeconds     int    `json:"window_seconds"`
}