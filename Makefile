.PHONY: run test test-stage

run:
	go run main.go

test:
	go test -v ./...

test-stage:
	go test -v ./... -run "$(STAGE)" -count=1