include standard.mk

CHECK_EXTRA := modverify

.PHONY: modverify integration workflows
modverify:
	$(GO) mod verify

integration:
	./script/integration -v

workflows:
	$(GO) run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
