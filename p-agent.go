package main

import (
	"peppa_hids/app"
	"time"
)

func main() {
	var agent app.Agent
	agent.Run()

	time.Sleep(10000000*time.Second)
}
