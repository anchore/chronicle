.PHONY: *
.DEFAULT_GOAL: make-default

make-default:
	@go run -C make .

.PHONY: *
.DEFAULT:
%:
	@go run -C make . $@
