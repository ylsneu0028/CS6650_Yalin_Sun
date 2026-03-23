package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type createCartRequest struct {
	CustomerID int32 `json:"customer_id" binding:"required"`
}

type createCartResponse struct {
	ShoppingCartID int64 `json:"shopping_cart_id"`
}

type addCartItemRequest struct {
	ProductID int32 `json:"product_id" binding:"required"`
	Quantity  int32 `json:"quantity" binding:"required"`
}

type cartItemJSON struct {
	ProductID int32 `json:"product_id"`
	Quantity  int32 `json:"quantity"`
}

type getCartResponse struct {
	ShoppingCartID int64          `json:"shopping_cart_id"`
	CustomerID     int32          `json:"customer_id"`
	Items          []cartItemJSON `json:"items"`
}

func parseShoppingCartID(c *gin.Context) (int64, bool) {
	raw := c.Param("shoppingCartId")
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < 1 {
		return 0, false
	}
	return v, true
}

func postShoppingCart(db *sql.DB) gin.HandlerFunc {
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

		res, err := db.ExecContext(c.Request.Context(),
			`INSERT INTO shopping_carts (customer_id) VALUES (?)`, body.CustomerID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to create shopping cart",
			})
			return
		}
		id, err := res.LastInsertId()
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to read new cart id",
			})
			return
		}
		c.JSON(http.StatusCreated, createCartResponse{ShoppingCartID: id})
	}
}

func getShoppingCart(db *sql.DB) gin.HandlerFunc {
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

		rows, err := db.QueryContext(c.Request.Context(), `
			SELECT c.id, c.customer_id, i.product_id, i.quantity
			FROM shopping_carts c
			LEFT JOIN cart_items i ON i.cart_id = c.id
			WHERE c.id = ?
		`, cartID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to load shopping cart",
			})
			return
		}
		defer rows.Close()

		var resp getCartResponse
		var items []cartItemJSON
		for rows.Next() {
			var cid int64
			var cust int32
			var pid sql.NullInt32
			var qty sql.NullInt32
			if err := rows.Scan(&cid, &cust, &pid, &qty); err != nil {
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "INTERNAL_ERROR",
					Message: "Internal server error",
					Details: "Failed to scan cart row",
				})
				return
			}
			resp.ShoppingCartID = cid
			resp.CustomerID = cust
			if pid.Valid && qty.Valid {
				items = append(items, cartItemJSON{ProductID: pid.Int32, Quantity: qty.Int32})
			}
		}
		if err := rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to iterate cart rows",
			})
			return
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

func postCartItems(db *sql.DB, store *Store) gin.HandlerFunc {
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

		tx, err := db.BeginTx(c.Request.Context(), nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to start transaction",
			})
			return
		}
		defer func() { _ = tx.Rollback() }()

		var existing int64
		err = tx.QueryRowContext(c.Request.Context(),
			`SELECT id FROM shopping_carts WHERE id = ? FOR UPDATE`, cartID).Scan(&existing)
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Shopping cart or product not found",
				Details: "Shopping cart does not exist",
			})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to verify shopping cart",
			})
			return
		}

		_, err = tx.ExecContext(c.Request.Context(), `
			INSERT INTO cart_items (cart_id, product_id, quantity) VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE quantity = VALUES(quantity)
		`, cartID, body.ProductID, body.Quantity)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to update cart items",
			})
			return
		}
		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to commit cart update",
			})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// DELETE /shopping-carts/:shoppingCartId/items/:productId — remove one line item (Part 2: item remove).
func deleteCartItem(db *sql.DB) gin.HandlerFunc {
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

		tx, err := db.BeginTx(c.Request.Context(), nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to start transaction",
			})
			return
		}
		defer func() { _ = tx.Rollback() }()

		var existing int64
		err = tx.QueryRowContext(c.Request.Context(),
			`SELECT id FROM shopping_carts WHERE id = ? FOR UPDATE`, cartID).Scan(&existing)
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Shopping cart not found",
				Details: "No shopping cart exists with the given id",
			})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to verify shopping cart",
			})
			return
		}

		res, err := tx.ExecContext(c.Request.Context(),
			`DELETE FROM cart_items WHERE cart_id = ? AND product_id = ?`, cartID, productID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to remove cart item",
			})
			return
		}
		n, err := res.RowsAffected()
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to confirm cart item removal",
			})
			return
		}
		if n == 0 {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Shopping cart or product not found",
				Details: "That product is not in this shopping cart",
			})
			return
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "Internal server error",
				Details: "Failed to commit cart update",
			})
			return
		}
		c.Status(http.StatusNoContent)
	}
}
