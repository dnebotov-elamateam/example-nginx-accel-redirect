package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	s3Endpoint := os.Getenv("S3_ENDPOINT")
	s3AccessKey := os.Getenv("S3_ACCESS_KEY")
	s3SecretKey := os.Getenv("S3_SECRET_KEY")
	s3FilePath := os.Getenv("S3_FILE_PATH")

	s3Client, err := minio.New(s3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(s3AccessKey, s3SecretKey, ""),
		Secure: false,
	})
	if err != nil {
		log.Fatalln(err)
	}

	httpEntrypoint := newEntrypoint(s3FilePath, s3Client)
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
	bucketName string
	filePath   string
	s3Client   *minio.Client
}

func newEntrypoint(filePath string, s3Client *minio.Client) *entrypoint {
	splitIndex := strings.Index(filePath, "/")

	return &entrypoint{
		bucketName: filePath[:splitIndex],
		filePath:   filePath[splitIndex:],
		s3Client:   s3Client,
	}
}

func (e *entrypoint) Registration(router *mux.Router) {
	router.HandleFunc("/api/v1/reports/file/{report-file-id}", e.reportFile)
}

func (e *entrypoint) reportFile(rw http.ResponseWriter, r *http.Request) {
	log.Println("INCOMING:", r.URL.String(), e.bucketName, e.filePath)

	object, err := e.s3Client.GetObject(r.Context(), e.bucketName, e.filePath, minio.GetObjectOptions{})
	if err != nil {
		log.Println("ERROR:", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer object.Close()

	info, err := object.Stat()
	if err != nil {
		log.Println("ERROR:", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", info.ContentType)
	rw.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", info.Key))

	if _, err = io.Copy(rw, object); err != nil {
		log.Println("ERROR:", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
}
