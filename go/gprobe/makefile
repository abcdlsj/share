all: build
	rm ${GOPATH}/bin/gprobe 2> /dev/null || true
	mv gprobe ${GOPATH}/bin/

build: 
	go build -o gprobe
