DIST_TEST_DIR = $(CURDIR)/../../../dist/test

.PHONY: all build build-debian-base build-base build-scion up down purge

all: up

build-debian-base:
	docker build -t debian-systemd:1.0 \
		-f $(DIST_TEST_DIR)/Dockerfile \
		$(DIST_TEST_DIR)

build-base%: build-debian-base
	docker build -t scion-base_ISD$*:1.0 \
		-f ./base_ISD$*/Dockerfile \
		./base_ISD$*

# pattern rule for scion01…scion05
build-scion%:
	docker build -t scion$*:1.0 \
		-f ./scion$*/Dockerfile \
		./scion$*

build%: build-base01 \
       build-scion01 build-scion02 build-scion03 \
       build-scion04 build-scion05 \
	   build-base02 \
	   build-scion06 build-scion07 build-scion08 \
       build-scion09 build-scion10

up: build
	docker compose up -d

down:
	docker compose down

purge: down
	docker ps -aq --filter "name=scion" | xargs -r docker rm -f
	docker network ls -q --filter "name=sciontutorial" | xargs -r docker network rm
