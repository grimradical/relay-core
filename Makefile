.PHONY: build
build:
	@./scripts/ci build

.PHONY: relay-%
relay-%:
	@./scripts/ci build $@
