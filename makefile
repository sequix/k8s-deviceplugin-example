build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dp -trimpath -ldflags "-s -w" cmd/main.go

docker: build
	bash ./script/build-image.sh

clean:
	rm -rf ./dp ./tmp
