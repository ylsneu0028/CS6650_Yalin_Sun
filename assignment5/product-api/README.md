Product API (Homework 5 – CS6650)

This project implements the Product API for a very simple e-commerce system, based strictly on the provided OpenAPI (api.yaml) specification.
Only the Products endpoints are implemented for this assignment.

The service supports:

Retrieving product details by product ID

Adding or updating product details

All product data is stored in memory using a hashmap, as required by the assignment.

Implemented Endpoints (Products Only)
GET /products/{productId}

Retrieve a product's details using its unique identifier.

Responses

200 OK – Product found

404 Not Found – Product does not exist

400 Bad Request – Invalid product ID

POST /products/{productId}/details

Add or update detailed information for a specific product.

Responses

204 No Content – Product details updated successfully

400 Bad Request – Invalid input data

404 Not Found – Product not found

Note: According to the OpenAPI specification, updating product details returns 204 No Content with no response body.

Data Model
Product
{
  "product_id": 1,
  "sku": "ABC-123-XYZ",
  "manufacturer": "Acme Corporation",
  "category_id": 456,
  "weight": 1250,
  "some_other_id": 789
}


All required fields and constraints (minimum values, string length limits) are validated exactly as specified in api.yaml.

Error Response
{
  "error": "INVALID_INPUT",
  "message": "Invalid input data",
  "details": "product_id must match path productId"
}

In-Memory Storage

Products are stored in a thread-safe in-memory hashmap (map[int32]Product)

The data does not persist across restarts

The server is initialized with a small set of preloaded products so that the update endpoint (POST /products/{productId}/details) can return 204 as defined in the specification

Running Locally (Without Docker)
Prerequisites

Go 1.21 or newer

Steps
go mod tidy
go run main.go


The server will start on:

http://localhost:8080

Running with Docker
Build the Docker image
docker build -t product-api .

Run the container
docker run --rm -p 8080:8080 product-api

API Usage Examples
Get an Existing Product
curl -i http://localhost:8080/products/1


Response

HTTP/1.1 200 OK
Content-Type: application/json

{
  "product_id": 1,
  "sku": "ABC-123-XYZ",
  "manufacturer": "Acme Corporation",
  "category_id": 456,
  "weight": 1250,
  "some_other_id": 789
}

Get a Non-Existing Product
curl -i http://localhost:8080/products/999


Response

HTTP/1.1 404 Not Found

{
  "error": "NOT_FOUND",
  "message": "Product not found"
}

Update Product Details
curl -i -X POST http://localhost:8080/products/1/details \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": 1,
    "sku": "ABC-123-XYZ",
    "manufacturer": "Acme Corporation",
    "category_id": 456,
    "weight": 1250,
    "some_other_id": 789
  }'


Response

HTTP/1.1 204 No Content

Invalid Input Example
curl -i -X POST http://localhost:8080/products/1/details \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": 2,
    "sku": "",
    "manufacturer": "Acme Corporation",
    "category_id": 456,
    "weight": 1250,
    "some_other_id": 789
  }'


Response

HTTP/1.1 400 Bad Request

{
  "error": "INVALID_INPUT",
  "message": "product_id mismatch",
  "details": "product_id must match path productId"
}

Notes on OpenAPI Compliance

All endpoints strictly follow the provided api.yaml contract

HTTP status codes match the specification exactly

Input validation enforces all required fields and constraints

Error responses conform to the defined Error schema