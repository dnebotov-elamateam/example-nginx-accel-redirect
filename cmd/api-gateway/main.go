package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"golang.org/x/sync/errgroup"
)

func main() {
	s3AccessKey := os.Getenv("S3_ACCESS_KEY")
	s3SecretKey := os.Getenv("S3_SECRET_KEY")
	s3Address := os.Getenv("S3_ADDRESS")

	log.Println(s3AccessKey)
	log.Println(s3SecretKey)
	log.Println(s3Address)

	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	client, err := minio.New(s3Address, &minio.Options{
		Creds:  credentials.NewStaticV4(s3AccessKey, s3SecretKey, ""),
		Secure: true,
	})
	if err != nil {
		log.Fatalln(err)
	}

	httpEntrypoint := newEntrypoint(client)
	httpRouter := mux.NewRouter()
	httpEntrypoint.Registration(httpRouter)

	httpServer := http.Server{
		Addr:    "0.0.0.0:9090",
		Handler: httpRouter,
	}

	gogroup, ctx := errgroup.WithContext(ctx)

	gogroup.Go(httpServer.ListenAndServe)
	<-ctx.Done()
	httpServer.Close()

	if err := gogroup.Wait(); err != nil {
		log.Panicln("ERROR:", err)
	}
}

type entrypoint struct {
	s3Client *minio.Client

	filePath string
}

func newEntrypoint(s3Client *minio.Client) *entrypoint {
	return &entrypoint{
		s3Client: s3Client,
		filePath: os.Getenv("S3_FILE_PATH"),
	}
}

func (e *entrypoint) Registration(router *mux.Router) {
	router.HandleFunc("/api/v1/reports/file/{report-file-id}", e.reportFile)
}

func (e *entrypoint) reportFile(rw http.ResponseWriter, r *http.Request) {
	log.Println("INCOMING:", r.URL.String())

	splitIndex := strings.Index(e.filePath, "/")

	bucket := e.filePath[:splitIndex]
	objectName := e.filePath[splitIndex+1:]
	fmt.Println(bucket, objectName)

	u, err := e.s3Client.PresignedGetObject(r.Context(), bucket, objectName, time.Minute, url.Values{})
	if err != nil {
		log.Fatalln(err)
	}

	accelRedirect := "/internal/report-files" + u.Path + "?" + u.RawQuery

	log.Println("URL:", accelRedirect)

	headers := rw.Header()
	headers.Add("X-Accel-Redirect", accelRedirect)
}
