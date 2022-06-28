package main

import (
	"context"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	s3AccessKey := os.Getenv("S3_ACCESS_KEY")
	s3SecretKey := os.Getenv("S3_SECRET_KEY")
	s3Address := os.Getenv("S3_ADDRESS")
	bucketName := os.Getenv("S3_BUCKET_NAME")

	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	client, err := minio.New(
		s3Address,
		&minio.Options{
			Creds: credentials.NewStaticV4(s3AccessKey, s3SecretKey, ""),
		},
	)
	if err != nil {
		log.Fatalln(err)
	}

	objs := client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{})

	for obj := range objs {
		link, err := client.PresignedGetObject(ctx, bucketName, obj.Key, time.Minute, url.Values{})
		if err != nil {
			log.Fatal(err)
		}
		log.Println(link.String())
	}
}
