
POLYGON_EDGE_BIN=.$(pwd)/third_party/polygon-edge/main
POLYGON_EDGE_DATA_DIR=$(pwd)/data
POLYGON_EDGE_CONFIGS_DIR=$(shell pwd)/configs

ifndef $(GOPATH)
    GOPATH=$(shell go env GOPATH)
    export GOPATH
endif

bootstrap-config:
	$(POLYGON_EDGE_BIN) server export --type yaml
	mv default-config.yaml configs/edge-config.yaml
	sed -i 's/genesis.json/configs\/genesis.json/g' configs/edge-config.yaml
	sed -i 's/log_level: INFO/log_level: DEBUG/g' configs/edge-config.yaml
	sed -i 's/data_dir: ""/data_dir: ".\/data\/avail-chain-1"/g' configs/edge-config.yaml

bootstrap-secrets:
	$(POLYGON_EDGE_BIN) secrets init --data-dir ./data/avail-bootnode-1
	$(POLYGON_EDGE_BIN) secrets init --data-dir ./data/avail-node-1
	$(POLYGON_EDGE_BIN) secrets init --data-dir ./data/avail-node-2

bootstrap-genesis:
	rm $(POLYGON_EDGE_CONFIGS_DIR)/genesis2.json || true
	$(POLYGON_EDGE_BIN) genesis --dir $(POLYGON_EDGE_CONFIGS_DIR)/genesis2.json \
	--name polygon-avail-settlement \
	--premine 0x064A4a5053F3de5eacF5E72A2E97D5F9CF55f031:1000000000000000000000 \
	--consensus ibft-avail \
	--bootnode /ip4/127.0.0.1/tcp/10001/p2p/16Uiu2HAmMNxPzdzkNmtV97e9Y7kvHWahpGysW2Mq7GdDCDFdAcZa \
	--ibft-validator 0x1bC763b9c36Bb679B17Fc9ed01Ec5e27AF145864

bootstrap: bootstrap-config bootstrap-secrets bootstrap-genesis

build-fraud-contract:
	solc --abi tools/fraud/contract/Fraud.sol -o tools/fraud/contract/ --overwrite
	solc --bin tools/fraud/contract/Fraud.sol -o tools/fraud/contract/ --overwrite
	abigen --bin=./tools/fraud/contract/Contract.bin --abi=./tools/fraud/contract/Contract.abi --pkg=fraud --out=./tools/fraud/contract/Fraud.go


build-server:
	cd server && go build -o server

build-client:
	cd client && go build -o client

build-e2e:
	cd tools/e2e && go build -o e2e

build-fraud: build-fraud-contract
	cd tools/fraud && go build -o fraud

build-contract:
	solc --abi contracts/SetGet/SetGet.sol -o contracts/SetGet/ --overwrite
	solc --bin contracts/SetGet/SetGet.sol -o contracts/SetGet/ --overwrite
	abigen --bin=./contracts/SetGet/SetGet.bin --abi=./contracts/SetGet/SetGet.abi --pkg=setget --out=./contracts/SetGet/SetGet.go


build-staking-contract:
	solc --abi contracts/staking/Staking.sol -o contracts/staking/ --overwrite
	solc --bin contracts/staking/Staking.sol -o contracts/staking/ --overwrite
	abigen --bin=./contracts/staking/Staking.bin --abi=./contracts/staking/Staking.abi --pkg=staking --out=./contracts/staking/Staking.go


build-edge:
	cd third_party/polygon-edge && make build

tools-wallet:
	cd tools/wallet && go build

build: build-server build-client

start-sequencer: build
	rm -rf data/avail-bootnode-1/blockchain/
	./server/server -config-file="./configs/bootnode.yaml"

start-validator: build
	rm -rf data/avail-node-1/blockchain/
	./server/server -config-file="./configs/node-1.yaml"

start-watchtower: build
	rm -rf data/avail-watchtower-1/blockchain/
	./server/server -config-file="./configs/watchtower-1.yaml"

start-e2e: build-e2e
	./tools/e2e/e2e


start-fraud: build-fraud
	./tools/fraud/fraud


deps:
ifeq (, $(shell which $(POLYGON_EDGE_BIN)))
	git submodule update --init third_party/polygon-edge
	cd third_party/polygon-edge && \
	make build && \
	mv main $(POLYGON_EDGE_BIN)
endif
	yarn install
	sh ./scripts/install_solc.sh

.PHONY: deps bootstrap