package store

import (
	"context"
	"errors"
	"fmt"
	"io"

	"golang.org/x/sync/errgroup"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/cs6650/album-store/internal/models"
)

var ErrNotFound = errors.New("not found")

type Config struct {
	Region      string
	AlbumsTable string
	PhotosTable string
	Bucket      string
}

type Store struct {
	cfg      *Config
	ddb      *dynamodb.Client
	s3       *s3.Client
	uploader *manager.Uploader
}

func New(cfg *Config, ddb *dynamodb.Client, s3c *s3.Client) *Store {
	up := manager.NewUploader(s3c, func(u *manager.Uploader) {
		u.PartSize = 16 * 1024 * 1024 // larger parts = fewer round-trips on big files (S12/S14/S15)
		u.Concurrency = 12
	})
	return &Store{cfg: cfg, ddb: ddb, s3: s3c, uploader: up}
}

func (s *Store) BucketName() string {
	return s.cfg.Bucket
}

type albumItem struct {
	AlbumID     string `dynamodbav:"album_id"`
	Title       string `dynamodbav:"title"`
	Description string `dynamodbav:"description"`
	Owner       string `dynamodbav:"owner"`
	LastSeq     int    `dynamodbav:"last_seq,omitempty"`
}

// PhotoItem is persisted metadata for a photo.
type PhotoItem struct {
	AlbumID   string `dynamodbav:"album_id"`
	PhotoID   string `dynamodbav:"photo_id"`
	Seq       int    `dynamodbav:"seq"`
	Status    string `dynamodbav:"status"`
	S3Key     string `dynamodbav:"s3_key,omitempty"`
	PublicURL string `dynamodbav:"public_url,omitempty"`
}

func (s *Store) PutAlbum(ctx context.Context, a *models.Album) error {
	item := albumItem{
		AlbumID:     a.AlbumID,
		Title:       a.Title,
		Description: a.Description,
		Owner:       a.Owner,
	}
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return err
	}
	_, err = s.ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.cfg.AlbumsTable),
		Item:      av,
	})
	return err
}

func (s *Store) GetAlbum(ctx context.Context, albumID string) (*models.Album, error) {
	out, err := s.ddb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.cfg.AlbumsTable),
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, ErrNotFound
	}
	var it albumItem
	if err := attributevalue.UnmarshalMap(out.Item, &it); err != nil {
		return nil, err
	}
	return &models.Album{
		AlbumID:     it.AlbumID,
		Title:       it.Title,
		Description: it.Description,
		Owner:       it.Owner,
	}, nil
}

func (s *Store) ListAlbums(ctx context.Context) ([]models.Album, error) {
	const segments = 8
	g, ctx := errgroup.WithContext(ctx)
	results := make([][]models.Album, segments)

	for seg := 0; seg < segments; seg++ {
		seg := seg
		g.Go(func() error {
			var local []models.Album
			var startKey map[string]types.AttributeValue
			ts := int32(segments)
			sg := int32(seg)
			for {
				out, err := s.ddb.Scan(ctx, &dynamodb.ScanInput{
					TableName:         aws.String(s.cfg.AlbumsTable),
					ExclusiveStartKey: startKey,
					Segment:           &sg,
					TotalSegments:     &ts,
				})
				if err != nil {
					return err
				}
				for _, item := range out.Items {
					var it albumItem
					if err := attributevalue.UnmarshalMap(item, &it); err != nil {
						return err
					}
					local = append(local, models.Album{
						AlbumID:     it.AlbumID,
						Title:       it.Title,
						Description: it.Description,
						Owner:       it.Owner,
					})
				}
				if out.LastEvaluatedKey == nil {
					break
				}
				startKey = out.LastEvaluatedKey
			}
			results[seg] = local
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	var all []models.Album
	for _, part := range results {
		all = append(all, part...)
	}
	return all, nil
}

// NextPhotoSeq atomically increments per-album sequence. Fails with ErrNotFound if album missing.
func (s *Store) NextPhotoSeq(ctx context.Context, albumID string) (int, error) {
	out, err := s.ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.cfg.AlbumsTable),
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
		},
		UpdateExpression: aws.String("ADD last_seq :one"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":one": &types.AttributeValueMemberN{Value: "1"},
		},
		ConditionExpression: aws.String("attribute_exists(album_id)"),
		ReturnValues:        types.ReturnValueAllNew,
	})
	if err != nil {
		var cfe *types.ConditionalCheckFailedException
		if errors.As(err, &cfe) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	var it albumItem
	if err := attributevalue.UnmarshalMap(out.Attributes, &it); err != nil {
		return 0, err
	}
	return it.LastSeq, nil
}

func (s *Store) PutPhoto(ctx context.Context, p *PhotoItem) error {
	av, err := attributevalue.MarshalMap(p)
	if err != nil {
		return err
	}
	_, err = s.ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.cfg.PhotosTable),
		Item:      av,
	})
	return err
}

func (s *Store) UpdatePhotoCompleted(ctx context.Context, albumID, photoID, s3Key, publicURL string) error {
	// Only transition processing→completed. If DELETE removed the row first, this must fail
	// without creating a new item (DynamoDB UpdateItem on missing item would otherwise upsert).
	_, err := s.ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.cfg.PhotosTable),
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
		UpdateExpression: aws.String("SET #s = :c, s3_key = :k, public_url = :u"),
		ConditionExpression: aws.String("#s = :p"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":c": &types.AttributeValueMemberS{Value: "completed"},
			":k": &types.AttributeValueMemberS{Value: s3Key},
			":u": &types.AttributeValueMemberS{Value: publicURL},
			":p": &types.AttributeValueMemberS{Value: "processing"},
		},
	})
	return err
}

func (s *Store) UpdatePhotoFailed(ctx context.Context, albumID, photoID string) error {
	_, err := s.ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.cfg.PhotosTable),
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
		UpdateExpression: aws.String("SET #s = :f"),
		ConditionExpression: aws.String("#s = :p"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":f": &types.AttributeValueMemberS{Value: "failed"},
			":p": &types.AttributeValueMemberS{Value: "processing"},
		},
	})
	return err
}

func (s *Store) fetchPhotoItem(ctx context.Context, albumID, photoID string) (*PhotoItem, error) {
	out, err := s.ddb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.cfg.PhotosTable),
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, ErrNotFound
	}
	var it PhotoItem
	if err := attributevalue.UnmarshalMap(out.Item, &it); err != nil {
		return nil, err
	}
	return &it, nil
}

func (s *Store) GetPhoto(ctx context.Context, albumID, photoID string) (*models.PhotoStatus, error) {
	it, err := s.fetchPhotoItem(ctx, albumID, photoID)
	if err != nil {
		return nil, err
	}
	ps := &models.PhotoStatus{
		PhotoID: it.PhotoID,
		AlbumID: it.AlbumID,
		Seq:     it.Seq,
		Status:  it.Status,
	}
	if it.Status == "completed" && it.PublicURL != "" {
		ps.URL = it.PublicURL
	}
	return ps, nil
}

func (s *Store) DeletePhotoRecord(ctx context.Context, albumID, photoID string) error {
	_, err := s.ddb.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.cfg.PhotosTable),
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
	})
	return err
}

func (s *Store) UploadObject(ctx context.Context, key string, body io.Reader, contentType string) error {
	in := &s3.PutObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
		Body:   body,
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	_, err := s.uploader.Upload(ctx, in)
	return err
}

func (s *Store) DeleteObject(ctx context.Context, key string) error {
	_, err := s.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	return err
}

// FetchPhotoItem loads raw metadata (used for DELETE to locate S3 key).
func (s *Store) FetchPhotoItem(ctx context.Context, albumID, photoID string) (*PhotoItem, error) {
	return s.fetchPhotoItem(ctx, albumID, photoID)
}

func PhotoObjectKey(albumID, photoID string) string {
	return fmt.Sprintf("albums/%s/photos/%s", albumID, photoID)
}

// PublicObjectURL builds the virtual-hosted–style HTTPS URL for a public object.
func PublicObjectURL(bucket, region, key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucket, region, key)
}
