build:
	dep ensure -v
	env GOOS=linux go build -ldflags="-s -w" -o bin/KanowinsHelp handlers/KanowinsHelp/main.go
	env GOOS=linux go build -ldflags="-s -w" -o bin/KanowinsCommand handlers/KanowinsCommand/main.go
	env GOOS=linux go build -ldflags="-s -w" -o bin/KanowinsInteractiveComponent handlers/KanowinsInteractiveComponent/main.go

.PHONY: clean
clean:
	rm -rf ./bin ./vendor Gopkg.lock

.PHONY: deploy
deploy: clean build
	sls deploy --verbose
