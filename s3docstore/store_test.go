package s3docstore

import (
	"flag"
	"fmt"
	"testing"

	"github.com/aefalcon/github-keystore-protobuf/go/appkeypb"
	"github.com/aefalcon/go-github-keystore/docstore"
	"github.com/aefalcon/go-github-keystore/kslog"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

var TestBucket string
var TestRegion string

const (
	FLAG_TEST_BUCKET = "test-bucket"
	FLAG_TEST_REGION = "test-region"
)

func init() {
	flag.StringVar(&TestBucket, FLAG_TEST_BUCKET, "", "S3 bucket from which to run tests")
	flag.StringVar(&TestRegion, FLAG_TEST_REGION, "us-east-1", "S3 bucket region")
}

func createTestBucket(client *s3.S3) error {
	input := s3.CreateBucketInput{
		Bucket: &TestBucket,
	}
	_, err := client.CreateBucket(&input)
	return err
}

func deleteTestBucket(client *s3.S3) error {
	listObjectsInput := s3.ListObjectsInput{
		Bucket: &TestBucket,
	}
	listObjectsResult, err := client.ListObjects(&listObjectsInput)
	if err != nil {
		return err
	}
	deleteObjects := make([]*s3.ObjectIdentifier, len(listObjectsResult.Contents))
	for i := range listObjectsResult.Contents {
		deleteObjects[i] = &s3.ObjectIdentifier{
			Key: listObjectsResult.Contents[i].Key,
		}
	}
	deleteObjectsInput := s3.DeleteObjectsInput{
		Bucket: &TestBucket,
		Delete: &s3.Delete{
			Objects: deleteObjects,
		},
	}
	_, err = client.DeleteObjects(&deleteObjectsInput)
	if err != nil {
		return err
	}
	input := s3.DeleteBucketInput{
		Bucket: &TestBucket,
	}
	_, err = client.DeleteBucket(&input)
	return err
}

func setUpBucketTest(t *testing.T) *s3.S3 {
	const flagReqMsg = "Flag -%s must be set"
	if TestBucket == "" {
		t.Fatalf(flagReqMsg, FLAG_TEST_BUCKET)
	}
	if TestRegion == "" {
		t.Fatalf(flagReqMsg, FLAG_TEST_REGION)
	}
	sess := session.Must(session.NewSession())
	client := s3.New(sess, aws.NewConfig().WithRegion(TestRegion))
	err := createTestBucket(client)
	if err != nil {
		t.Fatalf("Failed to create bucket: %s", err)
	}
	return client
}

func tearDownBucketTest(t *testing.T, client *s3.S3) error {
	err := deleteTestBucket(client)
	if err != nil {
		t.Logf("Failed to delete bucket: %s", err)
	}
	return err
}

func TestInitDb(t *testing.T) {
	client := setUpBucketTest(t)
	defer tearDownBucketTest(t, client)
	location := appkeypb.Location{
		Location: &appkeypb.Location_S3{
			S3: &appkeypb.S3Ref{
				Bucket: TestBucket,
				Region: TestRegion,
			},
		},
	}
	docStore, err := NewS3DocStore(&location)
	keyStore := docstore.AppKeyStore{
		DocStore: docStore,
		Links:    appkeypb.DefaultLinks,
	}
	if err != nil {
		t.Fatalf("Failed to create doc store: %s", err)
	}
	logger := kslog.KsTestLogger{
		TestLogger:  t,
		FailOnError: false,
	}
	err = keyStore.InitDb(&logger)
	if err != nil {
		t.Fatalf("Failed to initialize database: %s", err)
	}
}

func TestAddApp(t *testing.T) {
	client := setUpBucketTest(t)
	defer tearDownBucketTest(t, client)
	location := appkeypb.Location{
		Location: &appkeypb.Location_S3{
			S3: &appkeypb.S3Ref{
				Bucket: TestBucket,
				Region: TestRegion,
			},
		},
	}
	docStore, err := NewS3DocStore(&location)
	keyStore := docstore.AppKeyStore{
		DocStore: docStore,
		Links:    appkeypb.DefaultLinks,
	}
	if err != nil {
		t.Fatalf("Failed to create doc store: %s", err)
	}
	logger := kslog.KsTestLogger{
		TestLogger: t,
	}
	err = keyStore.InitDb(&logger)
	if err != nil {
		t.Fatalf("Failed to initialize database: %s", err)
	}
	testAddAppWithId := func(shouldPass bool, appId uint64, t *testing.T) {
		req := appkeypb.AddAppRequest{
			App: appId,
		}
		_, err = keyStore.AddApp(&req, &logger)
		if err != nil && shouldPass {
			t.Errorf("Failed to add app: %s", err)
		} else if err != nil && !shouldPass {
			// expected failure
		} else if err == nil && !shouldPass {
			t.Errorf("Test unexpectedly passed")
		} else if err == nil && shouldPass {
			// exected pass
		} else {
			panic("unexpected code path")
		}
	}
	testSpecs := []struct {
		appId         uint64
		shouldSucceed bool
	}{
		{0, false},
		{1, true},
		{2, true},
		{3, true},
	}
	for _, testSpec := range testSpecs {
		var stateMsg string
		if testSpec.shouldSucceed {
			stateMsg = "succeeds"
		} else {
			stateMsg = "fails"
		}
		testName := fmt.Sprintf("Add app %d %s", testSpec.appId, stateMsg)
		testFunc := func(t *testing.T) { testAddAppWithId(testSpec.shouldSucceed, testSpec.appId, t) }
		t.Run(testName, testFunc)
	}
}
