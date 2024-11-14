.PHONY: shivas
shivas:
	go build -o shivas main.go

.PHONY: test
test:
	go test ./...
	go test ../../libs/skylab/autotest/hostinfo/...

.PHONY: clean
clean:
	$(RM) shivas
