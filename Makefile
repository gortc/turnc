test-e2e:
	cd e2e/coturn && ./test.sh
lint:
	golangci-lint run
