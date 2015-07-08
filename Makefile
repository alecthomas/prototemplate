.PHONY: gen install

install: clean
	go install -ldflags "-X main.TemplateDir ${GOPATH}/src/github.com/alecthomas/prototemplate/templates" .

gen:
	protoc --gogo_out=./gen -I /usr/local/Cellar/protobuf/2.6.1/include /usr/local/Cellar/protobuf/2.6.1/include/google/protobuf/descriptor.proto

clean:
	rm -f ${GOPATH}/bin/prototemplate ${GOPATH}/pkg/darwin_amd64/github.com/alecthomas/prototemplate
