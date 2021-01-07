build:
	go build -o perftest ./main.go

run:
	./perftest -duration 3 -endpoint http://localhost:8000 -metrics http://localhost:9091 -vhosts 100