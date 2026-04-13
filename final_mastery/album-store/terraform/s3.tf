resource "aws_s3_bucket" "photos" {
  bucket = "${var.project}-photos-${data.aws_caller_identity.current.account_id}-${random_id.bucket_suffix.hex}"

  # Allow terraform destroy to delete non-empty bucket (photo objects)
  force_destroy = true
}

resource "aws_s3_bucket_public_access_block" "photos" {
  bucket = aws_s3_bucket.photos.id

  block_public_acls       = false
  block_public_policy     = false
  ignore_public_acls      = false
  restrict_public_buckets = false
}

resource "aws_s3_bucket_ownership_controls" "photos" {
  bucket = aws_s3_bucket.photos.id
  rule {
    object_ownership = "BucketOwnerPreferred"
  }
}

resource "aws_s3_bucket_policy" "photos_public_read" {
  bucket = aws_s3_bucket.photos.id
  depends_on = [
    aws_s3_bucket_public_access_block.photos,
    aws_s3_bucket_ownership_controls.photos,
  ]

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "PublicReadGetObject"
        Effect    = "Allow"
        Principal = "*"
        Action    = "s3:GetObject"
        Resource  = "${aws_s3_bucket.photos.arn}/*"
      },
    ]
  })
}
