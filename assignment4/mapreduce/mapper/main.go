package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type MapResponse struct {
	Chunk     string `json:"chunk_uri"`
	MapOutput string `json:"map_output"`
	NumWords  int    `json:"num_words"`
	NumKeys   int    `json:"num_unique_words"`
}

func parseS3URI(s3uri string) (bucket, key string, err error) {
	if !strings.HasPrefix(s3uri, "s3://") {
		return "", "", fmt.Errorf("invalid s3 uri (must start with s3://): %s", s3uri)
	}
	u, err := url.Parse(s3uri)
	if err != nil {
		return "", "", err
	}
	bucket = u.Host
	key = strings.TrimPrefix(u.Path, "/")
	if bucket == "" || key == "" {
		return "", "", fmt.Errorf("invalid s3 uri (missing bucket or key): %s", s3uri)
	}
	return bucket, key, nil
}

// tokenize: lower-case, keep only a-z and 0-9 as words (simple + consistent)
var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

func tokenizeLine(line string) []string {
	line = strings.ToLower(line)
	line = nonWord.ReplaceAllString(line, " ")
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	return strings.Fields(line)
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

	// GET /map?chunk_uri=s3://bucket/chunks/xxx.chunk0.txt&out_prefix=map
	r.GET("/map", func(c *gin.Context) {
		chunkURI := c.Query("chunk_uri")
		outPrefix := c.DefaultQuery("out_prefix", "map")

		if chunkURI == "" {
			c.JSON(400, gin.H{"error": "missing query param: chunk_uri"})
			return
		}

		bucket, key, err := parseS3URI(chunkURI)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		obj, err := s3c.GetObject(ctx, &s3.GetObjectInput{
			Bucket: &bucket,
			Key:    &key,
		})
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("GetObject failed: %v", err)})
			return
		}
		defer obj.Body.Close()

		counts := map[string]int{}
		totalWords := 0

		sc := bufio.NewScanner(obj.Body)
		buf := make([]byte, 0, 64*1024)
		sc.Buffer(buf, 2*1024*1024)

		for sc.Scan() {
			words := tokenizeLine(sc.Text())
			for _, w := range words {
				counts[w]++
				totalWords++
			}
		}
		if err := sc.Err(); err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("read chunk failed: %v", err)})
			return
		}

		// optional: add a little metadata for debugging
		meta := map[string]any{
			"chunk_uri":        chunkURI,
			"total_words":      totalWords,
			"unique_words":     len(counts),
			"generated_at_utc": time.Now().UTC().Format(time.RFC3339),
			"counts":           counts,
		}

		body, err := json.Marshal(meta)
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("json marshal failed: %v", err)})
			return
		}

		// output key: map/<chunk_base>.json
		base := path.Base(key)
		if base == "" || base == "." || base == "/" {
			base = "chunk.txt"
		}
		// keep stable name + timestamp to avoid overwriting (immutable tag in ECR doesn't matter here, but S3 overwrites by default)
		ts := time.Now().UTC().Format("20060102T150405Z")
		outKey := fmt.Sprintf("%s/%s.%s.json", strings.TrimSuffix(outPrefix, "/"), base, ts)

		_, err = s3c.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      &bucket,
			Key:         &outKey,
			Body:        bytes.NewReader(body),
			ContentType: strPtr("application/json"),
		})
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("PutObject failed: %v", err)})
			return
		}

		c.JSON(200, MapResponse{
			Chunk:     chunkURI,
			MapOutput: fmt.Sprintf("s3://%s/%s", bucket, outKey),
			NumWords:  totalWords,
			NumKeys:   len(counts),
		})
	})

	// optional debug endpoint: show top-k words quickly (not required)
	r.GET("/top", func(c *gin.Context) {
		chunkURI := c.Query("chunk_uri")
		kStr := c.DefaultQuery("k", "20")
		k, _ := strconv.Atoi(kStr)
		if k <= 0 {
			k = 20
		}
		if chunkURI == "" {
			c.JSON(400, gin.H{"error": "missing query param: chunk_uri"})
			return
		}

		bucket, key, err := parseS3URI(chunkURI)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		obj, err := s3c.GetObject(ctx, &s3.GetObjectInput{
			Bucket: &bucket,
			Key:    &key,
		})
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("GetObject failed: %v", err)})
			return
		}
		defer obj.Body.Close()

		counts := map[string]int{}
		sc := bufio.NewScanner(obj.Body)
		buf := make([]byte, 0, 64*1024)
		sc.Buffer(buf, 2*1024*1024)
		for sc.Scan() {
			for _, w := range tokenizeLine(sc.Text()) {
				counts[w]++
			}
		}
		type kv struct {
			W string
			C int
		}
		var arr []kv
		for w, c0 := range counts {
			arr = append(arr, kv{w, c0})
		}
		sort.Slice(arr, func(i, j int) bool { return arr[i].C > arr[j].C })
		if len(arr) > k {
			arr = arr[:k]
		}
		c.JSON(200, arr)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	_ = http.ListenAndServe(":"+port, r)
}

func strPtr(s string) *string { return &s }
