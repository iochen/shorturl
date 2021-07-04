package main

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type COS struct {
	EndPoint   string `yaml:"end_point"`
	AccessKey  string `yaml:"access_key"`
	SecretKey  string `yaml:"secret_key"`
	UseSSL     bool   `yaml:"use_ssl"`
	BucketName string `yaml:"bucket_name"`
}

type MinIOClient struct {
	*minio.Client
	bucket string
}

func (mio *MinIOClient) Bucket() string {
	return mio.bucket
}

func (cos *COS) NewMinIO() (*MinIOClient, error) {
	client, err := minio.New(cos.EndPoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cos.AccessKey, cos.SecretKey, ""),
		Secure: cos.UseSSL})
	if err != nil {
		return &MinIOClient{}, err
	}

	exists, err := client.BucketExists(context.Background(), cos.BucketName)
	if err != nil {
		return &MinIOClient{}, err
	}

	if !exists {
		return &MinIOClient{}, fmt.Errorf("bucket %s not exists", cos.BucketName)
	}

	return &MinIOClient{client, cos.BucketName}, nil
}
