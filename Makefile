.SILENT: install-swagger

install-swagger:
	go get -u github.com/swaggo/swag/cmd/swag 2>/dev/null && go install github.com/swaggo/swag/cmd/swag@latest
	$$GOPATH/bin/swag -v

swag-gen: install-swagger
	$$GOPATH/bin/swag fmt && $$GOPATH/bin/swag init --parseDependency --parseInternal --parseDepth 1

errcheck:
	go get -u github.com/kisielk/errcheck 2>/dev/null && go install github.com/kisielk/errcheck@latest
	$$GOPATH/bin/errcheck ./...

goconst:
	go get -u github.com/jgautheron/goconst/cmd/goconst 2>/dev/null && go install github.com/jgautheron/goconst/cmd/goconst@latest
	$$GOPATH/bin/goconst ./...

gocyclo:
	go get -u  github.com/fzipp/gocyclo/cmd/gocyclo 2>/dev/null && go install  github.com/fzipp/gocyclo/cmd/gocyclo@latest
	$$GOPATH/bin/gocyclo -avg .

all-check: errcheck goconst gocyclo