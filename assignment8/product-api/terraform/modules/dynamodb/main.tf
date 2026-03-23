# Single-table design: pk + sk
# SYSTEM / CART_COUNTER — atomic counter for numeric cart ids (API-compatible with MySQL)
# CART#<id> / META — customer_id, created_at
# CART#<id> / ITEM#<productId> — quantity

resource "aws_dynamodb_table" "shopping_carts" {
  name         = "${var.name_prefix}-shopping-carts"
  billing_mode = "PAY_PER_REQUEST"

  hash_key  = "pk"
  range_key = "sk"

  attribute {
    name = "pk"
    type = "S"
  }

  attribute {
    name = "sk"
    type = "S"
  }

  tags = {
    Name = "${var.name_prefix}-shopping-carts"
  }
}
