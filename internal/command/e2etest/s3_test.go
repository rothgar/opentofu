// Copyright (c) OpenTofu
// SPDX-License-Identifier: MPL-2.0
// Package s3 contains the integration tests for 's3' backend
package e2etest

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/opentofu/opentofu/internal/e2e"
)

func TestInitBackendS3(t *testing.T) {
	skipIfNotEnvVarSet(t)

	const (
		bucket = "foo-bar"
		region = "eu-north-1"
	)

	t.Run("shall succeed given bucket and prefix in the main.tf", func(t *testing.T) {
		fixturePath := t.TempDir()

		content := fmt.Sprintf(`terraform {
			backend "s3" {
				bucket   = "%s"
				key      = "%s/terraform.tfstate"
				region   = "%s"
		}
	}`, bucket, newS3Prefix(), region,
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
		TODO: Given bucket, prefix and custom endpoint in the main.tf, When run tofu init+tofu plan, Then the state object exists in the bucket;
		TODO: Given empty “backend” config block, When run tofu init --cfg=.. + tofu plan, Then the state object exists in the bucket;
		TODO: Given empty “backend” config block, When run tofu init --cfg=.. twice, Then the state object exists in the bucket.
	*/
}

func skipIfNotEnvVarSet(t *testing.T) {
	if os.Getenv("TF_E2E_S3") != "1" {
		t.Skip("Skipping test, required environment variables missing. Use `TF_E2E_S3`=1 to run")
	}
}

func newS3Prefix() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}
