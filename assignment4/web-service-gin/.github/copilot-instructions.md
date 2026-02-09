# Copilot Instructions: Album Web Service (Gin)

## Project Overview
This is a simple REST API service built with **Gin Web Framework** (Go) that manages album records. The service runs on port 8080 and provides three endpoints for creating, reading, and retrieving album data. It's containerized with Docker and deployable to AWS ECR.

**Key Components:**
- **main.go**: REST API with Gin router, album data model, and three HTTP handlers
- **Dockerfile**: Multi-stage build (Go 1.25.5) for Linux containers with static binary compilation
- **go.mod/go.sum**: Dependency management (Gin framework v1.11.0)

## Architecture & Data Flow
1. **Album Model**: Simple struct with JSON serialization tags (id, title, artist, price)
2. **In-Memory Storage**: `albums` slice initialized with seed data (3 default albums)
3. **HTTP Endpoints**:
   - `GET /albums` → returns all albums as JSON with 200 OK
   - `GET /albums/:id` → returns single album or 404 if not found
   - `POST /albums` → binds JSON request body to new album, appends to slice, returns 201 Created

## Build & Deployment Workflow
- **Local Testing**: `go run main.go` starts server on :8080
- **Docker Build**: Uses CGO_ENABLED=0 GOOS=linux for static binary compilation (platform-independent)
- **ECR Deployment**: `docker buildx build --builder singlearch --platform linux/amd64 --push -t $ECR_URL .`
  - Note: Binary output name is `/docker-gs-ping` (defined in Dockerfile line 17)

## Code Patterns & Conventions
- **Error Handling**: Minimal - POST returns early on BindJSON error without explicit error response
- **HTTP Status Codes**: Uses appropriate codes (200 OK, 201 Created, 404 Not Found)
- **JSON Serialization**: Struct tags with lowercase field names for API compatibility
- **Gin Handlers**: Accept `*gin.Context` receiver for HTTP operations

## Important Implementation Details
- **Album Search**: Linear search in getAlbumByID (O(n)) - acceptable for seed data size
- **No Persistence**: Data is volatile in-memory; resets on server restart
- **Hardcoded Port**: Service listens on `:8080` - cannot be configured via env vars or flags
- **Gin Mode**: Uses `gin.Default()` which includes logging and recovery middleware

## Common Issues & Gotchas
1. **Missing go.sum**: Dockerfile expects go.sum to exist; run `go mod tidy` if missing
2. **Port Binding**: Must specify port when running Docker container: `docker run -p 8080:8080 <image>`
3. **Binary Name Mismatch**: Dockerfile builds `/docker-gs-ping` but go.mod declares module as "hello-server"
4. **POST Validation**: No explicit validation of required fields; empty/invalid JSON silently accepted

## Testing Commands
```bash
# Start server locally
go run main.go

# Test GET all albums
curl http://localhost:8080/albums

# Test GET single album
curl http://localhost:8080/albums/1

# Test POST new album
curl -X POST http://localhost:8080/albums \
  -H "Content-Type: application/json" \
  -d '{"id":"4","title":"Kind of Blue","artist":"Miles Davis","price":12.99}'

# Build Docker image
docker build -t album-service:latest .

# Run Docker container
docker run -p 8080:8080 album-service:latest
```

## When Modifying Code
- **Adding Routes**: Edit main.go router.GET/POST lines; keep consistent with Gin conventions
- **Changing Response Format**: Modify album struct JSON tags and c.IndentedJSON calls
- **Adding Dependencies**: Run `go get <module>`, commit go.mod/go.sum; rebuild Docker image
- **Cross-Platform Deployment**: Keep CGO_ENABLED=0 and GOOS=linux flags in Dockerfile
