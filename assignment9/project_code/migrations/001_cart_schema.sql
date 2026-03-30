-- Shopping cart schema for RDS MySQL (also applied automatically on app startup).
-- Indexes: customer_id for history queries; cart_id on line items for JOINs;
-- PK (cart_id, product_id) supports upsert and prevents duplicate rows per product.

CREATE TABLE IF NOT EXISTS shopping_carts (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  customer_id INT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_customer_id (customer_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS cart_items (
  cart_id BIGINT NOT NULL,
  product_id INT NOT NULL,
  quantity INT NOT NULL,
  PRIMARY KEY (cart_id, product_id),
  CONSTRAINT fk_cart_items_cart FOREIGN KEY (cart_id) REFERENCES shopping_carts(id) ON DELETE CASCADE,
  KEY idx_cart_id (cart_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
