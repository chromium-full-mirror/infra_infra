.PHONY: lsnexus
lsnexus:
	@$(RM) -rf ./lsnexus
	go test ./...
	env CGO_ENABLED=0 go build -o ./lsnexus -- .

.PHONY: test
test:
	go test ./...

.PHONY: clean
clean:
	$(RM) lsnexus