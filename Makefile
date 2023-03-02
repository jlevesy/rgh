.PHONY: gateway
gateway:
	go run ./cmd/gateway \
		-name gateway-0 \
		-gossip-port 7948 \
		-coap-port 10000 \
		-join-address 127.0.0.1:7946

.PHONY: debug-gateway
debug-gateway:
	dlv debug ./cmd/gateway -- \
		-name gateway-0 \
		-gossip-port 7948 \
		-coap-port 10000 \
		-join-address 127.0.0.1:7946

.PHONY: members
members:
	go run ./cmd/memberdump \
		-name memberdump \
		-gossip-port 7945 \
		-join-address 127.0.0.1:7946

.PHONY: replica-0
replica-0:
	go run ./cmd/server \
		-name replica-0 \
		-gossip-port 7946 \
		-coap-port 10001 \
		-join-address 127.0.0.1

.PHONY: replica-1
replica-1:
	go run ./cmd/server \
		-name replica-1 \
		-gossip-port 7947 \
		-coap-port 10002 \
		-join-address 127.0.0.1:7946
