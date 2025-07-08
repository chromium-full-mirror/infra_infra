.PHONY: lsnexus_testing
lsnexus_testing:
	@$(RM) -rf ./lsnexus_testing
	go test ./...
	env CGO_ENABLED=0 go build -o ./lsnexus_testing -- .

.PHONY: test
test:
	go test ./...

.PHONY: clean
clean:

