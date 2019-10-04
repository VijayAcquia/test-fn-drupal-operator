export GO111MODULE=auto

lint:
	gofmt -l . | tee $(BUFFER)
	@! test -s $(BUFFER)
	helm init --client-only > /dev/null
	cd helm \
	  && ./package.sh \
	  && cd fn-drupal-operator \
	  && helm lint

lint-fix:
	gofmt -w .

BUFFER := $(shell mktemp)

.PHONY: lint lint-fix
