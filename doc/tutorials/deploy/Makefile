DIST_TEST_DIR = $(CURDIR)/../../../dist/test

.PHONY: all build build-debian-base build-base build-scion up down purge

all: up

build-debian-base:
	docker build -t debian-systemd:1.0 \
		-f $(DIST_TEST_DIR)/Dockerfile \
		$(DIST_TEST_DIR)

build-base%: build-debian-base
	docker build -t scion-base-isd$*:1.0 \
		-f ./base-isd$*/Dockerfile \
		./base-isd$*

# Each scion image depends on its base image
build-scion01 build-scion02 build-scion03 build-scion04 build-scion05: build-base01
build-scion06 build-scion07 build-scion08 build-scion09 build-scion10: build-base02

# pattern rule for scion01…scion05
build-scion%:
	docker build -t scion$*:1.0 \
		-f ./scion$*/Dockerfile \
		./scion$*

# Main build target with proper ordering
build: build-base01 build-base02 \
       build-scion01 build-scion02 build-scion03 \
       build-scion04 build-scion05 \
       build-scion06 build-scion07 build-scion08 \
       build-scion09 build-scion10

route-isd01-%: docker exec scion$* ip route add 10.200.0.0/24 via 10.100.0.254
route-isd02-%: docker exec scion$* ip route add 10.100.0.0/24 via 10.200.0.254

up: build
	docker compose up -d
	route-isd01-01 route-isd01-02 route-isd01-03 route-isd01-04 route-isd01-05 \
	route-isd02-06 route-isd02-07 route-isd02-08 route-isd02-09 route-isd02-10 \

down:
	docker compose down

purge: down
	docker ps -aq --filter "name=scion" | xargs -r docker rm -f
	docker network ls -q --filter "name=sciontutorial" | xargs -r docker network rm
