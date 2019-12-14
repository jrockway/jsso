package main

import (
	"github.com/jrockway/jsso/lib/server"
)

func main() {
	server.AppName = "envoy-plugin"
	server.Setup()
	server.ListenAndServe()
}
