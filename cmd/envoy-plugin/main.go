package main

import (
	auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v2"
	"github.com/jrockway/jsso/lib/authserver"
	"github.com/jrockway/jsso/lib/server"
	"google.golang.org/grpc"
)

func main() {
	server.AppName = "envoy-plugin"
	server.Setup()
	authServer := &authserver.Server{}
	server.AddService(func(s *grpc.Server) {
		auth.RegisterAuthorizationServer(s, authServer)
	})
	server.ListenAndServe()
}
