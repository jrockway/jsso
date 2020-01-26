// Package authserver implements an Envoy ext_authz service.
package authserver

import (
	"context"

	envoyauth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v2"
	envoy_type "github.com/envoyproxy/go-control-plane/envoy/type"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/jrockway/jsso/lib/auth"
	"go.uber.org/zap"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	PolicyServer *auth.Server
}

func allow() *envoyauth.CheckResponse {
	return &envoyauth.CheckResponse{
		Status: &rpcstatus.Status{
			Code: int32(codes.OK),
		},
		HttpResponse: &envoyauth.CheckResponse_OkResponse{
			OkResponse: &envoyauth.OkHttpResponse{},
		},
	}
}

func deny(msg string) *envoyauth.CheckResponse {
	return &envoyauth.CheckResponse{
		Status: &rpcstatus.Status{
			Code: int32(codes.PermissionDenied),
		},
		HttpResponse: &envoyauth.CheckResponse_DeniedResponse{
			DeniedResponse: &envoyauth.DeniedHttpResponse{
				Status: &envoy_type.HttpStatus{
					Code: envoy_type.StatusCode_Forbidden,
				},
				Body: msg,
			},
		},
	}
}

// Check envoyauthorizes a request.
func (s *Server) Check(ctx context.Context, req *envoyauth.CheckRequest) (*envoyauth.CheckResponse, error) {
	id := req.GetAttributes().GetRequest().GetHttp().GetHeaders()["x-request-id"]
	logger := ctxzap.Extract(ctx).With(zap.String("x-request-id", id))

	ok, err := s.PolicyServer.Eval(ctx, req)
	if err != nil {
		logger.Error("eval policy", zap.Error(err))
		return deny(err.Error()), status.Error(codes.Internal, err.Error())
	}
	logger.Debug("access checked", zap.Bool("allowed", ok))
	if ok {
		return allow(), nil
	}
	return deny("Access denied: policy requirements not met."), nil
}
