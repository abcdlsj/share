all: build
	rm ${GOPATH}/bin/gresh 2> /dev/null || true
	mv gresh ${GOPATH}bin/

build: 
	go build -o gresh
