# team-sample-app — local dev ergonomics for the Phase-1 golden path.
# The CI seam (build/push/bump) is driven by .devops/promotion.yaml (§4.1).
.DEFAULT_GOAL := help

ENV  ?= dev
CI    = .devops/ci

.PHONY: help test app-build app-push bump deploy render

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | awk 'BEGIN{FS=":.*?## "}{printf "  %-12s %s\n", $$1, $$2}'

test: ## Run the app unit tests
	cd app && go test ./...

app-build: ## Build + push the app image to the k3d registry for ENV (default dev). Prints IMAGE/TAG.
	$(CI)/build-and-push.sh $(ENV)

app-push: app-build ## Alias for app-build (build implies push)

bump: ## Bump the ENV overlay image tag: make bump ENV=dev TAG=abc1234 [COMMIT=1]
	@test -n "$(TAG)" || { echo "set TAG=<tag>"; exit 2; }
	COMMIT=$(COMMIT) $(CI)/bump-image.sh $(ENV) $(TAG)

deploy: ## Full local loop for ENV: build+push, then bump+commit (the GitOps signal)
	@IMG=$$($(CI)/build-and-push.sh $(ENV) | sed -n 's/^TAG=//p'); \
	  COMMIT=1 $(CI)/bump-image.sh $(ENV) $$IMG

render: ## Render every overlay (validation, no cluster needed)
	@for e in dev staging prod preview; do \
	  kubectl kustomize .devops/chart/overlays/$$e >/dev/null && echo "$$e OK" || exit 1; \
	done
