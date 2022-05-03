bin_dir=$(shell pwd)/bin

bin/protoc-gen-twirp: ../go.mod
	GOBIN=$(bin_dir) go install github.com/twitchtv/twirp/protoc-gen-twirp

bin/protoc-gen-go: ../go.mod
	GOBIN=$(bin_dir) go install github.com/golang/protobuf/protoc-gen-go

rpc/testservice/service.pb.go: bin/protoc-gen-twirp bin/protoc-gen-go
	PATH="$(bin_dir):$(PATH)" protoc \
		-I rpc \
		--go_out=rpc \
		--twirp_out=rpc \
		rpc/testservice/service.proto
