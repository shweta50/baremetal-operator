package token

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/platform9/pf9-qbert/sunpike/conductor/pkg/api"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	cacheValidityHours = 10
)

type keystoneAuthResult struct {
	ProjectID string `json:"projectID"`
	Token     string `json:"token"`
}

// defaultConfig contains sane defaults
var defaultConfig = grpcConfig{
	TransportURL:            "localhost:8111",
	ConnectTimeout:          20,
	GRPCRetryMax:            3,
	GRPCRetryTimeoutSeconds: 5,
}

var tokenCache = keystoneAuthResult{}

type grpcConfig struct {
	ClusterID               string
	ProjectID               string
	TransportURL            string
	ConnectTimeout          int
	GRPCRetryMax            uint
	GRPCRetryTimeoutSeconds int
	conn                    api.ConductorClient
}

func getKeystoneToken(ctx context.Context, clusterID, project string) (*keystoneAuthResult, error) {

	defaultConfig.ClusterID = clusterID
	defaultConfig.ProjectID = project

	// Make rpc call to sunpike conductor through comms
	keystoneAuthResult, err := fetchSunpikeAuthInfo(ctx, &defaultConfig)
	if err != nil {
		log.Errorf("Error fetching keystone token: %s", err)
		if tokenCache.Token != "" && tokenCache.ProjectID != "" {
			log.Info("Using cached token")
			return &tokenCache, nil
		}

		return nil, err
	}

	if tokenCache.Token != keystoneAuthResult.Token {
		tokenCache.Token = keystoneAuthResult.Token
		tokenCache.ProjectID = keystoneAuthResult.ProjectID
	}

	return &tokenCache, nil
}

func fetchSunpikeAuthInfo(ctx context.Context, cfg *grpcConfig) (*keystoneAuthResult, error) {

	cl, err := createSunpikeClient(cfg)
	if err != nil {
		log.Errorf("Error creating sunpike client: %s", err)
		return nil, err
	}

	authRequestObj := api.AuthRequest{ClusterID: cfg.ClusterID, ProjectID: cfg.ProjectID}
	timeout := time.Duration(cfg.ConnectTimeout) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	callOptions := getGRPCOptions(cfg)

	var keystoneAuthResult keystoneAuthResult
	resp, err := cl.FetchSunpikeAuthInfo(ctx, &authRequestObj, callOptions...)
	if err != nil {
		return nil, fmt.Errorf("error sending auth info request to sunpike: %v", err)
	}

	err = json.Unmarshal([]byte(resp.AuthInfo), &keystoneAuthResult)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling FetchSunpikeAuthInfo into KeystoneAuthResult %v", err)
	}

	return &keystoneAuthResult, nil
}

func createSunpikeClient(cfg *grpcConfig) (api.ConductorClient, error) {
	timeout := time.Duration(cfg.ConnectTimeout) * time.Second
	/*
	* WithInsecure - No auth; We rely on comms <-> haproxy tunnel for mutual TLS auth.
	* WithBlock - Synchronous gRPC connection creation; so that we can fail faster
	* WithTimeout - We should not wait forever for blocking connection to be completed. Default is 20s.
	 */
	conn, err := grpc.Dial(cfg.TransportURL, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(timeout))
	if err != nil {
		log.Errorf("Error dialing sunpike grpc: %s", err)
		return nil, err
	}
	return api.NewConductorClient(conn), nil
}

func getGRPCOptions(cfg *grpcConfig) []grpc.CallOption {
	/*
	* Configures max retries to be cfg.GRPCRetryCount with deadline of each attempt will be now + cfg.GRPCRetryTimeout
	* Retries will only be attempted on "Aborted", "Unavailable", "Cancelled" and "DeadlineExceeded" status codes.
	* "WithPerRetryTimeout" handles the "Cancelled" and "DeadlineExceeded" codes.
	 */
	grpcOpts := []grpc.CallOption{
		grpc_retry.WithMax(cfg.GRPCRetryMax),
		grpc_retry.WithPerRetryTimeout(time.Duration(cfg.GRPCRetryTimeoutSeconds) * time.Second),
		grpc_retry.WithCodes(codes.Aborted, codes.Unavailable),
	}
	return grpcOpts
}
