package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/sts"
)

type SnapshotRetriever interface {
	GetSnapshots(clusterName, region string) ([]*ec2.Snapshot, error)
}

type DefaultSnapshotRetriever struct {
}

func NewDefaultSnapshotRetriever() *DefaultSnapshotRetriever {
	return &DefaultSnapshotRetriever{}
}

func getAWSIdentity(s *session.Session) (*sts.GetCallerIdentityOutput, error) {
	svc := sts.New(s)
	identity, err := svc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, err
	}

	return identity, nil
}

func (r *DefaultSnapshotRetriever) GetSnapshots(clusterName, region string) ([]*ec2.Snapshot, error) {
	s := session.Must(session.NewSession(&aws.Config{Region: aws.String(region)}))

	identity, err := getAWSIdentity(s)
	if err != nil {
		return nil, err
	}

	svc := ec2.New(s)
	input := &ec2.DescribeSnapshotsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:kubernetes.io/cluster/" + clusterName),
				Values: []*string{aws.String("owned")},
			},
		},
		OwnerIds: []*string{identity.Account},
	}

	result, err := svc.DescribeSnapshots(input)
	if err != nil {
		return nil, err
	}

	return result.Snapshots, nil
}
