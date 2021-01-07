run:
	go run main.go

docker-build:
	docker build -t xenitab/gatekeeper-exporter .
