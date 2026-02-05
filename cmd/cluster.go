package cmd

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
)

// ClusterInfo represents the essential information needed for a Kubernetes cluster
type ClusterInfo struct {
	Name            string
	Endpoint        string
	CertificateData []byte
	Region          string
	Arn             string
	AuthCommand     string
	AuthArgs        []string
	AuthEnv         map[string]string
}

// ClusterProvider defines the interface for discovering and describing clusters
type ClusterProvider interface {
	ListClusters(ctx context.Context, profile string) ([]string, error)
	GetClusterInfo(ctx context.Context, profile, clusterName string) (*ClusterInfo, error)
	GetRegion(ctx context.Context, profile string) (string, error)
}

// AWSClusterProvider implements ClusterProvider for AWS EKS
type AWSClusterProvider struct{}

func (p *AWSClusterProvider) ListClusters(ctx context.Context, profile string) ([]string, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	eksClient := eks.NewFromConfig(cfg)
	clustersOutput, err := eksClient.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list EKS clusters: %w", err)
	}
	return clustersOutput.Clusters, nil
}

func (p *AWSClusterProvider) GetClusterInfo(ctx context.Context, profile, clusterName string) (*ClusterInfo, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	eksClient := eks.NewFromConfig(cfg)
	clusterOutput, err := eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe EKS cluster: %w", err)
	}

	cluster := clusterOutput.Cluster
	if cluster.CertificateAuthority == nil || cluster.CertificateAuthority.Data == nil {
		return nil, fmt.Errorf("cluster certificate authority data is nil")
	}

	ca, err := base64.StdEncoding.DecodeString(*cluster.CertificateAuthority.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode certificate authority data: %w", err)
	}

	region := cfg.Region

	return &ClusterInfo{
		Name:            *cluster.Name,
		Endpoint:        *cluster.Endpoint,
		CertificateData: ca,
		Region:          region,
		Arn:             *cluster.Arn,
		AuthCommand:     "aws",
		AuthArgs: []string{
			"eks",
			"get-token",
			"--cluster-name", *cluster.Name,
			"--region", region,
		},
		AuthEnv: map[string]string{
			"AWS_PROFILE": profile,
		},
	}, nil
}

func (p *AWSClusterProvider) GetRegion(ctx context.Context, profile string) (string, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}
	return cfg.Region, nil
}

// ManualClusterProvider allows manually specifying cluster information
// This will be useful when you want to move away from AWS discovery
type ManualClusterProvider struct {
	clusters map[string]*ClusterInfo
}

func NewManualClusterProvider() *ManualClusterProvider {
	return &ManualClusterProvider{
		clusters: make(map[string]*ClusterInfo),
	}
}

func (p *ManualClusterProvider) AddCluster(info *ClusterInfo) {
	p.clusters[info.Name] = info
}

func (p *ManualClusterProvider) ListClusters(ctx context.Context, profile string) ([]string, error) {
	var names []string
	for name := range p.clusters {
		names = append(names, name)
	}
	return names, nil
}

func (p *ManualClusterProvider) GetClusterInfo(ctx context.Context, profile, clusterName string) (*ClusterInfo, error) {
	info, exists := p.clusters[clusterName]
	if !exists {
		return nil, fmt.Errorf("cluster %s not found in manual configuration", clusterName)
	}
	return info, nil
}

func (p *ManualClusterProvider) GetRegion(ctx context.Context, profile string) (string, error) {
	// For manual provider, we could return a default region or extract from cluster info
	return "manual", nil
}
