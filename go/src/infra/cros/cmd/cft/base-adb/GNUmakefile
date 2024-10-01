.PHONY: adb
adb:	
	@$(RM) -rf ./base-adb
	go test ./...
	env CGO_ENABLED=0 go build -o ./base-adb -- .

.PHONY: test
test:
	go test ./...

.PHONY: clean
clean:
	$(RM) adb