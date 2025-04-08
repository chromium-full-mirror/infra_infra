.PHONY: bols_satlab
bols_satlab:
	@$(RM) -rf ./bols_satlab
	go test ./...
	env CGO_ENABLED=0 go build -o ./bols_satlab -- .

.PHONY: test
test:
	go test ./...

.PHONY: clean
clean:
	$(RM) bols_satlab