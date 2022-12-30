gen:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative grpc/interface.proto
client0:
	go run main.go 0
	
client1:
	go run main.go 1

client2:
	go run main.go 2

