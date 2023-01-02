

clean:
	rm -rf ./bin/container

build:
	go build -o ./bin/container 
	
deploy:
	make clean
	make build