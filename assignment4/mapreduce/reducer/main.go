package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ReduceResponse struct {
	MapURIs       []string `json:"map_uris"`
	ReduceOutput  string   `json:"reduce_output"`
	TotalKeys     int      `json:"total_unique_words"`
	GeneratedAtUT string   `json:"generated_at_utc"`
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

// Mapper output json includes: {"counts": {"word": 12, ...}, ...}
type MapperJSON struct {
	Counts map[string]int `json:"counts"`
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

	// GET /reduce?map_uris=s3://...json,s3://...json&out_prefix=reduce
	r.GET("/reduce", func(c *gin.Context) {
		mapUrisRaw := c.Query("map_uris")
		outPrefix := c.DefaultQuery("out_prefix", "reduce")

		if mapUrisRaw == "" {
			c.JSON(400, gin.H{"error": "missing query param: map_uris"})
			return
		}

		parts := strings.Split(mapUrisRaw, ",")
		var mapURIs []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				mapURIs = append(mapURIs, p)
			}
		}
		if len(mapURIs) == 0 {
			c.JSON(400, gin.H{"error": "map_uris is empty"})
			return
		}

		// We'll write output to the bucket of the first map uri (simple).
		outBucket, _, err := parseS3URI(mapURIs[0])
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		final := map[string]int{}

		// read each mapper json and aggregate
		for _, u := range mapURIs {
			b, k, err := parseS3URI(u)
			if err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
			obj, err := s3c.GetObject(ctx, &s3.GetObjectInput{
				Bucket: &b,
				Key:    &k,
			})
			if err != nil {
				c.JSON(500, gin.H{"error": fmt.Sprintf("GetObject failed for %s: %v", u, err)})
				return
			}

			var mj MapperJSON
			dec := json.NewDecoder(obj.Body)
			if err := dec.Decode(&mj); err != nil {
				_ = obj.Body.Close()
				c.JSON(500, gin.H{"error": fmt.Sprintf("decode json failed for %s: %v", u, err)})
				return
			}
			_ = obj.Body.Close()

			for w, cnt := range mj.Counts {
				final[w] += cnt
			}
		}

		// build final json
		output := map[string]any{
			"map_uris":          mapURIs,
			"total_unique":      len(final),
			"generated_at_utc":  time.Now().UTC().Format(time.RFC3339),
			"counts":            final,
			"tokenize_strategy": "lowercase + non[a-z0-9]=>space + fields",
		}
		body, err := json.Marshal(output)
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("json marshal failed: %v", err)})
			return
		}

		ts := time.Now().UTC().Format("20060102T150405Z")
		outKey := fmt.Sprintf("%s/final.%s.json", strings.TrimSuffix(outPrefix, "/"), ts)

		_, err = s3c.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      &outBucket,
			Key:         &outKey,
			Body:        bytes.NewReader(body),
			ContentType: strPtr("application/json"),
		})
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("PutObject failed: %v", err)})
			return
		}

		c.JSON(200, ReduceResponse{
			MapURIs:       mapURIs,
			ReduceOutput:  fmt.Sprintf("s3://%s/%s", outBucket, outKey),
			TotalKeys:     len(final),
			GeneratedAtUT: time.Now().UTC().Format(time.RFC3339),
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	_ = http.ListenAndServe(":"+port, r)
}

func strPtr(s string) *string { return &s }
