package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	httpEntrypoint := newEntrypoint()
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
	s3AccessKey string
	s3SecretKey string
	filePath    string
}

func newEntrypoint() *entrypoint {
	return &entrypoint{
		s3AccessKey: os.Getenv("S3_ACCESS_KEY"),
		s3SecretKey: os.Getenv("S3_SECRET_KEY"),
		filePath:    os.Getenv("S3_FILE_PATH"),
	}
}

func (e *entrypoint) Registration(router *mux.Router) {
	router.HandleFunc("/api/v1/reports/file/{report-file-id}", e.reportFile)
}

func (e *entrypoint) reportFile(rw http.ResponseWriter, r *http.Request) {
	log.Println("INCOMING:", r.URL.String())

	accelRedirect := "/internal/report-files" + e.filePath
	_, filename := filepath.Split(e.filePath)
	ext := filepath.Ext(filename)
	date := time.Now().Format(time.RFC1123Z)
	var contentType string

	switch ext {
	case "xlsx":
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}

	signaturePayload := fmt.Sprintf(
		"GET\n\n%s\n\nx-amz-date:%s\n%s",
		contentType,
		date,
		e.filePath,
	)
	signature := GetSignature(signaturePayload, e.s3SecretKey)
	authorization := fmt.Sprintf("AWS4 %s:%s", e.s3AccessKey, signature)

	log.Println("OUTGOING:", "X-Accel-Redirect", accelRedirect, "X-Authorization", authorization, "X-Filename", filename)

	headers := rw.Header()
	headers.Add("X-Accel-Redirect", accelRedirect)
	headers.Add("X-Authorization", authorization)
	headers.Add("X-Filename", filename)
}

func GetSignature(input, key string) string {
	keyForSign := []byte(key)
	h := hmac.New(sha256.New, keyForSign)
	h.Write([]byte(input))

	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
