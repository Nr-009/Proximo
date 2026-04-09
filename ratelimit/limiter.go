package ratelimit

type Limiter interface {
    Allow(clientPort string) bool
    Done(clientPort string)
}
