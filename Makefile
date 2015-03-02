install:
	go install -ldflags "-X main.TemplateDir ${GOPATH}/src/github.com/alecthomas/prototemplate/templates" .
