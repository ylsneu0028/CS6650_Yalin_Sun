package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/sync/semaphore"

	"github.com/cs6650/album-store/internal/handlers"
	"github.com/cs6650/album-store/internal/store"
)

func main() {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-west-2"
	}
	albumsTable := os.Getenv("ALBUMS_TABLE")
	photosTable := os.Getenv("PHOTOS_TABLE")
	bucket := os.Getenv("S3_BUCKET")
	if albumsTable == "" || photosTable == "" || bucket == "" {
		log.Fatal("ALBUMS_TABLE, PHOTOS_TABLE, and S3_BUCKET must be set")
	}

	cfg, err := awsconfig.LoadDefaultConfig(nil, awsconfig.WithRegion(region))
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	st := store.New(&store.Config{
		Region:      region,
		AlbumsTable: albumsTable,
		PhotosTable: photosTable,
		Bucket:      bucket,
	}, dynamodb.NewFromConfig(cfg), s3.NewFromConfig(cfg))

	uploadConc := int64(64)
	if v := os.Getenv("UPLOAD_CONCURRENCY"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			uploadConc = n
		}
	}

	h := &handlers.Handler{
		Store:       st,
		Region:      region,
		UploadSlots: semaphore.NewWeighted(uploadConc),
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	if os.Getenv("ACCESS_LOG") != "0" {
		r.Use(middleware.Logger)
	}

	r.Get("/health", h.Health)
	r.Get("/albums", h.ListAlbums)
	r.Put("/albums/{album_id}", h.PutAlbum)
	r.Get("/albums/{album_id}", h.GetAlbum)
	r.Post("/albums/{album_id}/photos", h.UploadPhoto)
	r.Get("/albums/{album_id}/photos/{photo_id}", h.GetPhoto)
	r.Delete("/albums/{album_id}/photos/{photo_id}", h.DeletePhoto)

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       0, // large multipart uploads (S15)
		WriteTimeout:      0,
		IdleTimeout:       120 * time.Second,
	}
	log.Printf("listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
