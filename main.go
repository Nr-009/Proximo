package main

import (
	"github.com/Nr-009/Proximo/proxy"
)

func main() {
	p := proxy.New()
	p.Start(8080, 9000)
}