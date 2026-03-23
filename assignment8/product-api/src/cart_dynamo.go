package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gin-gonic/gin"
)

const (
	dynamoSKMeta         = "META"
	dynamoPKCounter      = "SYSTEM"
	dynamoSKCounter      = "CART_COUNTER"
	dynamoPKCartPrefix   = "CART#"
	dynamoSKItemPrefix   = "ITEM#"
)

type dynamoCartStore struct {
	client *dynamodb.Client
	table  string
}

func newDynamoCartStore(ctx context.Context, region, table string) (*dynamoCartStore, error) {
	if table == "" {
		return nil, fmt.Errorf("DYNAMODB_TABLE_NAME is empty")
	}
	if region == "" {
		region = "us-west-2"
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}
	return &dynamoCartStore{
		client: dynamodb.NewFromConfig(cfg),
		table:  table,
	}, nil
}

func cartPartitionKey(cartID int64) string {
	return fmt.Sprintf("%s%d", dynamoPKCartPrefix, cartID)
}

func itemSortKey(productID int32) string {
	return fmt.Sprintf("%s%d", dynamoSKItemPrefix, productID)
}

// nextCartID atomically increments a counter (numeric ids match MySQL-style API).
func (s *dynamoCartStore) nextCartID(ctx context.Context) (int64, error) {
	out, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: dynamoPKCounter},
			"sk": &types.AttributeValueMemberS{Value: dynamoSKCounter},
		},
		UpdateExpression: aws.String("ADD next_id :one"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":one": &types.AttributeValueMemberN{Value: "1"},
		},
		ReturnValues: types.ReturnValueUpdatedNew,
	})
	if err != nil {
		return 0, err
	}
	n, ok := out.Attributes["next_id"].(*types.AttributeValueMemberN)
	if !ok || n == nil {
		return 0, fmt.Errorf("counter returned unexpected shape")
	}
	return strconv.ParseInt(n.Value, 10, 64)
}

func (s *dynamoCartStore) putMeta(ctx context.Context, cartID int64, customerID int32) error {
	pk := cartPartitionKey(cartID)
	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item: map[string]types.AttributeValue{
			"pk":          &types.AttributeValueMemberS{Value: pk},
			"sk":          &types.AttributeValueMemberS{Value: dynamoSKMeta},
			"customer_id": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", customerID)},
			"created_at":  &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
		},
	})
	return err
}

func (s *dynamoCartStore) getMeta(ctx context.Context, cartID int64) (customerID int32, ok bool, err error) {
	pk := cartPartitionKey(cartID)
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
			"sk": &types.AttributeValueMemberS{Value: dynamoSKMeta},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return 0, false, err
	}
	if out.Item == nil || len(out.Item) == 0 {
		return 0, false, nil
	}
	cattr, okc := out.Item["customer_id"].(*types.AttributeValueMemberN)
	if !okc {
		return 0, false, fmt.Errorf("META missing customer_id")
	}
	cid, err := strconv.ParseInt(cattr.Value, 10, 32)
	if err != nil {
		return 0, false, err
	}
	return int32(cid), true, nil
}

func postShoppingCartDynamo(s *dynamoCartStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body createCartRequest
		if err := c.ShouldBindJSON(&body); err != nil || body.CustomerID < 1 {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "The provided input data is invalid",
				Details: "customer_id must be a positive integer",
			})
			return
		}

		id, err := s.nextCartID(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to allocate cart id",
			})
			return
		}
		if err := s.putMeta(c.Request.Context(), id, body.CustomerID); err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to create shopping cart",
			})
			return
		}
		c.JSON(http.StatusCreated, createCartResponse{
			ShoppingCartID: id,
			CustomerID:     body.CustomerID,
			Items:          []cartItemJSON{},
		})
	}
}

func getShoppingCartDynamo(s *dynamoCartStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cartID, ok := parseShoppingCartID(c)
		if !ok {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "The provided input data is invalid",
				Details: "shoppingCartId must be a positive integer",
			})
			return
		}

		pk := cartPartitionKey(cartID)
		out, err := s.client.Query(c.Request.Context(), &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			KeyConditionExpression: aws.String("pk = :pk"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk": &types.AttributeValueMemberS{Value: pk},
			},
			ConsistentRead: aws.Bool(true),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to load shopping cart",
			})
			return
		}

		var resp getCartResponse
		var items []cartItemJSON
		for _, it := range out.Items {
			sk, ok := it["sk"].(*types.AttributeValueMemberS)
			if !ok {
				continue
			}
			switch {
			case sk.Value == dynamoSKMeta:
				cattr, okc := it["customer_id"].(*types.AttributeValueMemberN)
				if !okc {
					continue
				}
				cid, err := strconv.ParseInt(cattr.Value, 10, 32)
				if err != nil {
					continue
				}
				resp.CustomerID = int32(cid)
				resp.ShoppingCartID = cartID
			case strings.HasPrefix(sk.Value, dynamoSKItemPrefix):
				pidStr := strings.TrimPrefix(sk.Value, dynamoSKItemPrefix)
				pid64, err := strconv.ParseInt(pidStr, 10, 32)
				if err != nil {
					continue
				}
				qattr, okq := it["quantity"].(*types.AttributeValueMemberN)
				if !okq {
					continue
				}
				qty, err := strconv.ParseInt(qattr.Value, 10, 32)
				if err != nil {
					continue
				}
				items = append(items, cartItemJSON{ProductID: int32(pid64), Quantity: int32(qty)})
			}
		}
		if resp.ShoppingCartID == 0 {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Shopping cart not found",
				Details: "No shopping cart exists with the given id",
			})
			return
		}
		resp.Items = items
		c.JSON(http.StatusOK, resp)
	}
}

func postCartItemsDynamo(s *dynamoCartStore, store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		cartID, ok := parseShoppingCartID(c)
		if !ok {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "The provided input data is invalid",
				Details: "shoppingCartId must be a positive integer",
			})
			return
		}

		var body addCartItemRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "The provided input data is invalid",
				Details: "Request body must include product_id and quantity",
			})
			return
		}
		if body.ProductID < 1 || body.Quantity < 1 {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "The provided input data is invalid",
				Details: "product_id and quantity must be positive integers",
			})
			return
		}
		if _, exists := store.Get(body.ProductID); !exists {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Shopping cart or product not found",
				Details: "Product does not exist",
			})
			return
		}

		_, metaOK, err := s.getMeta(c.Request.Context(), cartID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to verify shopping cart",
			})
			return
		}
		if !metaOK {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Shopping cart or product not found",
				Details: "Shopping cart does not exist",
			})
			return
		}

		pk := cartPartitionKey(cartID)
		sk := itemSortKey(body.ProductID)
		_, err = s.client.PutItem(c.Request.Context(), &dynamodb.PutItemInput{
			TableName: aws.String(s.table),
			Item: map[string]types.AttributeValue{
				"pk":        &types.AttributeValueMemberS{Value: pk},
				"sk":        &types.AttributeValueMemberS{Value: sk},
				"quantity":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", body.Quantity)},
				"product_id": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", body.ProductID)},
			},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to update cart items",
			})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func deleteCartItemDynamo(s *dynamoCartStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cartID, ok := parseShoppingCartID(c)
		if !ok {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "The provided input data is invalid",
				Details: "shoppingCartId must be a positive integer",
			})
			return
		}

		raw := c.Param("productId")
		prodID64, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || prodID64 < 1 {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "The provided input data is invalid",
				Details: "productId must be a positive integer",
			})
			return
		}
		productID := int32(prodID64)

		_, metaOK, err := s.getMeta(c.Request.Context(), cartID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to verify shopping cart",
			})
			return
		}
		if !metaOK {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Shopping cart not found",
				Details: "No shopping cart exists with the given id",
			})
			return
		}

		pk := cartPartitionKey(cartID)
		sk := itemSortKey(productID)
		out, err := s.client.DeleteItem(c.Request.Context(), &dynamodb.DeleteItemInput{
			TableName: aws.String(s.table),
			Key: map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberS{Value: pk},
				"sk": &types.AttributeValueMemberS{Value: sk},
			},
			ReturnValues: types.ReturnValueAllOld,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to remove cart item",
			})
			return
		}
		if out.Attributes == nil {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Shopping cart or product not found",
				Details: "That product is not in this shopping cart",
			})
			return
		}
		c.Status(http.StatusNoContent)
	}
}
