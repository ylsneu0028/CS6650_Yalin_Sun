resource "aws_dynamodb_table" "albums" {
  name         = "${var.project}-albums"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "album_id"

  attribute {
    name = "album_id"
    type = "S"
  }
}

resource "aws_dynamodb_table" "photos" {
  name         = "${var.project}-photos"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "album_id"
  range_key    = "photo_id"

  attribute {
    name = "album_id"
    type = "S"
  }

  attribute {
    name = "photo_id"
    type = "S"
  }
}
