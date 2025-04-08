.PHONY: bols_labstation
bols_labstation:
	@$(RM) -rf ./bols_labstation
	go test ./...
	env CGO_ENABLED=0 go build -o ./bols_labstation -- .

.PHONY: test
test:
	go test ./...

.PHONY: clean
clean:
	$(RM) bols_labstation