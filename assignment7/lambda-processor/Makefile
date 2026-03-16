# Build Lambda deployment zip for Linux (custom runtime provided.al2023 expects "bootstrap")
.PHONY: build clean
build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bootstrap main.go
	zip -j deployment.zip bootstrap

clean:
	rm -f bootstrap deployment.zip
