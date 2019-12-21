package main

import (
	"context"
	"io/ioutil"
	"time"

	envoy_auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v2"
	"github.com/jrockway/jsso/lib/auth"
	"github.com/jrockway/jsso/lib/authserver"
	"github.com/jrockway/jsso/lib/server"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type flags struct {
	PolicyPath string `long:"policy" env:"POLICY_PATH" default:"policy.rego" description:"location of OPA policy to load"`
}

func main() {
	server.AppName = "envoy-plugin"
	f := new(flags)
	server.AddFlagGroup("Main", f)
	server.Setup()

	policyServer := auth.New()
	policy, err := ioutil.ReadFile(f.PolicyPath)
	if err != nil {
		zap.L().Fatal("read policy file", zap.String("policy_file", f.PolicyPath), zap.Error(err))
	}

	ctx, c := context.WithTimeout(context.Background(), 5*time.Second)
	if err := policyServer.LoadPolicy(ctx, string(policy)); err != nil {
		zap.L().Fatal("load policy file", zap.String("policy_file", f.PolicyPath), zap.Error(err))
	}
	c()

	authServer := &authserver.Server{PolicyServer: policyServer}
	server.AddService(func(s *grpc.Server) {
		envoy_auth.RegisterAuthorizationServer(s, authServer)
	})
	server.ListenAndServe()
}
