.PHONY: gen install

install:
	go install -ldflags "-X main.TemplateDir ${GOPATH}/src/github.com/alecthomas/prototemplate/templates" .

gen:
	protoc --gogo_out=./gen -I /usr/local/Cellar/protobuf/2.6.1/include /usr/local/Cellar/protobuf/2.6.1/include/google/protobuf/descriptor.proto
