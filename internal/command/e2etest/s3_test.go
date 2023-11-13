// Copyright (c) OpenTofu
// SPDX-License-Identifier: MPL-2.0
// Package s3 contains the integration tests for 's3' backend
package e2etest

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	awsbase "github.com/hashicorp/aws-sdk-go-base/v2"
	baselogging "github.com/hashicorp/aws-sdk-go-base/v2/logging"
	"github.com/opentofu/opentofu/internal/e2e"
	"github.com/opentofu/opentofu/internal/logging"
)

func TestInitBackendS3(t *testing.T) {
	skipIfNotEnvVarSet(t)

	const (
		bucketPrefix = "org.opentofu.state.e2etest"
		region       = "eu-north-1"
	)

	ctx := context.TODO()

	ctx, baselog := attachLoggerToContext(ctx)
	cfg := &awsbase.Config{
		Region: region,
		Logger: baselog,
	}
	_, awsConfig, awsDiags := awsbase.GetAwsConfig(ctx, cfg)

	if awsDiags.HasError() {
		t.Fatalf("failed to init s3 client")
	}

	s3Client := s3.NewFromConfig(awsConfig)

	t.Run("shall succeed given bucket and prefix in the main.tf", func(t *testing.T) {
		bucket := bucketPrefix + newS3Suffix()

		createS3Bucket(ctx, t, s3Client, bucket, region)
		defer deleteS3Bucket(ctx, t, s3Client, bucket)

		fixturePath := t.TempDir()

		content := fmt.Sprintf(`terraform {
			backend "s3" {
				bucket   = "%s"
				key      = "terraform.tfstate"
				region   = "%s"
		}
	}`, bucket, region,
		)

		if err := os.WriteFile(path.Join(fixturePath, "main.tf"), []byte(content), 0644); err != nil {
			t.Fatalf("cannot write main.tf, err: %v\n", err)
		}

		tf := e2e.NewBinary(t, terraformBin, fixturePath)

		stdout, stderr, err := tf.Run("init")
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}

		if stderr != "" {
			t.Errorf("unexpected stderr output:\n%s", stderr)
		}

		if !strings.Contains(stdout, "OpenTofu has been successfully initialized!") {
			t.Errorf("success message is missing from output:\n%s", stdout)
		}
	})

	/*
		TODO: https://github.com/opentofu/opentofu/issues/819
		TODO: Given bucket, prefix and custom endpoint in the main.tf, When run tofu init, Then the state object exists in the bucket;
		TODO: Given empty “backend” config block, When run tofu init --cfg=.., Then the state object exists in the bucket;
		TODO: Given empty “backend” config block, When run tofu init --cfg=.. twice, Then the state object exists in the bucket.
	*/
}

func skipIfNotEnvVarSet(t *testing.T) {
	if os.Getenv("TF_E2E_S3") != "1" {
		t.Skip("Skipping test, required environment variables missing. Use `TF_E2E_S3`=1 to run")
	}
}

func newS3Suffix() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

func createS3Bucket(ctx context.Context, t *testing.T, s3Client *s3.Client, bucketName, region string) {
	createBucketReq := &s3.CreateBucketInput{
		Bucket: &bucketName,
	}

	// Regions outside of us-east-1 require the appropriate LocationConstraint
	// to be specified in order to create the bucket in the desired region.
	// https://docs.aws.amazon.com/cli/latest/reference/s3api/create-bucket.html
	if region != "us-east-1" {
		createBucketReq.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(region),
		}
	}

	// Be clear about what we're doing in case the user needs to clean
	// this up later.
	t.Logf("creating S3 bucket %s in %s", bucketName, region)
	_, err := s3Client.CreateBucket(ctx, createBucketReq)
	if err != nil {
		t.Fatal("failed to create test S3 bucket:", err)
	}
}

func deleteS3Bucket(ctx context.Context, t *testing.T, s3Client *s3.Client, bucketName string) {
	warning := "WARNING: Failed to delete the test S3 bucket. It may have been left in your AWS account and may incur storage charges. (error was %s)"

	// first we have to get rid of the env objects, or we can't delete the bucket
	resp, err := s3Client.ListObjects(ctx, &s3.ListObjectsInput{Bucket: &bucketName})
	if err != nil {
		t.Logf(warning, err)
		return
	}
	for _, obj := range resp.Contents {
		if _, err := s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &bucketName, Key: obj.Key}); err != nil {
			// this will need cleanup no matter what, so just warn and exit
			t.Logf(warning, err)
			return
		}
	}

	if _, err := s3Client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: &bucketName}); err != nil {
		t.Logf(warning, err)
	}
}

func attachLoggerToContext(ctx context.Context) (context.Context, baselogging.HcLogger) {
	ctx, baselog := baselogging.NewHcLogger(ctx, logging.HCLogger().Named("e2e-s3-test"))
	ctx = baselogging.RegisterLogger(ctx, baselog)
	return ctx, baselog
}
