package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type SplitResponse struct {
	Input  string   `json:"s3_uri"`
	N      int      `json:"n"`
	Chunks []string `json:"chunks"`
}

func parseS3URI(s3uri string) (bucket, key string, err error) {
	if !strings.HasPrefix(s3uri, "s3://") {
		return "", "", fmt.Errorf("invalid s3_uri (must start with s3://): %s", s3uri)
	}
	u, err := url.Parse(s3uri)
	if err != nil {
		return "", "", err
	}
	bucket = u.Host
	key = strings.TrimPrefix(u.Path, "/")
	if bucket == "" || key == "" {
		return "", "", fmt.Errorf("invalid s3_uri (missing bucket or key): %s", s3uri)
	}
	return bucket, key, nil
}

func main() {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-west-2"
	}

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		panic(fmt.Errorf("load aws config: %w", err))
	}
	s3c := s3.NewFromConfig(cfg)

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.String(200, "ok")
	})

	r.GET("/split", func(c *gin.Context) {
		s3uri := c.Query("s3_uri")
		nStr := c.DefaultQuery("n", "3")
		outPrefix := c.DefaultQuery("out_prefix", "chunks")

		n, err := strconv.Atoi(nStr)
		if err != nil || n <= 0 {
			c.JSON(400, gin.H{"error": "n must be a positive integer"})
			return
		}
		if s3uri == "" {
			c.JSON(400, gin.H{"error": "missing query param: s3_uri"})
			return
		}

		bucket, key, err := parseS3URI(s3uri)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		// 1) Download object
		obj, err := s3c.GetObject(ctx, &s3.GetObjectInput{
			Bucket: &bucket,
			Key:    &key,
		})
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("GetObject failed: %v", err)})
			return
		}
		defer obj.Body.Close()

		// 2) Read all lines
		var lines []string
		sc := bufio.NewScanner(obj.Body)
		// Increase scanner buffer for large lines
		buf := make([]byte, 0, 64*1024)
		sc.Buffer(buf, 2*1024*1024)
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
		if err := sc.Err(); err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("read input failed: %v", err)})
			return
		}
		if len(lines) == 0 {
			c.JSON(400, gin.H{"error": "input file is empty"})
			return
		}

		// 3) Split lines evenly
		chunkSize := (len(lines) + n - 1) / n

		base := path.Base(key)
		if base == "" || base == "/" || base == "." {
			base = "input.txt"
		}
		// add a timestamp to avoid overwriting
		ts := time.Now().UTC().Format("20060102T150405Z")

		var chunkURIs []string

		for i := 0; i < n; i++ {
			start := i * chunkSize
			if start >= len(lines) {
				break
			}
			end := (i + 1) * chunkSize
			if end > len(lines) {
				end = len(lines)
			}

			var b bytes.Buffer
			for _, line := range lines[start:end] {
				b.WriteString(line)
				b.WriteString("\n")
			}

			outKey := fmt.Sprintf("%s/%s.%s.chunk%d.txt", strings.TrimSuffix(outPrefix, "/"), base, ts, i)
			body := b.Bytes()

			_, err := s3c.PutObject(ctx, &s3.PutObjectInput{
				Bucket: &bucket,
				Key:    &outKey,
				Body:   bytes.NewReader(body),
			})
			if err != nil {
				c.JSON(500, gin.H{"error": fmt.Sprintf("PutObject failed: %v", err)})
				return
			}

			chunkURIs = append(chunkURIs, fmt.Sprintf("s3://%s/%s", bucket, outKey))
		}

		c.JSON(200, SplitResponse{
			Input:  s3uri,
			N:      n,
			Chunks: chunkURIs,
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	_ = http.ListenAndServe(":"+port, r)
}
