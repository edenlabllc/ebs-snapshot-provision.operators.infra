package aws

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Type "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"golang.org/x/net/context"
)

type EC2Client struct {
	Ctx       context.Context
	EC2Client *ec2.Client
	*IAMEC2Metadata
}

type IAMEC2Metadata struct {
	AccountID string
	Region    string
}

type SnapshotRetriever interface {
	GetAWSSnapshots(clusterName, region string) ([]ec2Type.Snapshot, error)
}

type DefaultAWSSnapshotRetriever struct {
}

func NewDefaultAWSSnapshotRetriever() *DefaultAWSSnapshotRetriever {
	return &DefaultAWSSnapshotRetriever{}
}

func newAWSEC2Client(region string) (*EC2Client, error) {
	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region), config.WithRetryer(
		func() aws.Retryer {
			return retry.NewStandard(func(o *retry.StandardOptions) { o.MaxAttempts = 10 })
		},
	))
	if err != nil {
		return nil, err
	}

	identity, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, err
	}

	return &EC2Client{
		Ctx:            ctx,
		EC2Client:      ec2.NewFromConfig(cfg),
		IAMEC2Metadata: &IAMEC2Metadata{AccountID: aws.ToString(identity.Account), Region: region},
	}, nil
}

func (sr *DefaultAWSSnapshotRetriever) GetAWSSnapshots(clusterName, region string) ([]ec2Type.Snapshot, error) {
	client, err := newAWSEC2Client(region)
	if err != nil {
		return nil, err
	}

	input := &ec2.DescribeSnapshotsInput{
		Filters: []ec2Type.Filter{
			{
				Name:   aws.String("tag:kubernetes.io/cluster/" + clusterName),
				Values: []string{"owned"},
			},
		},
		OwnerIds: []string{client.AccountID},
	}

	result, err := client.EC2Client.DescribeSnapshots(client.Ctx, input)
	if err != nil {
		return nil, err
	}

	return result.Snapshots, nil
}
