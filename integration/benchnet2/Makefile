# eventually, DOCKER_TAG will use the git commit hash
# this isn't working fully yet so fow now we will specify the explicit Git tag to use as the Docker tag
#DOCKER_TAG := $(shell git rev-parse --short HEAD)
FLOW_GO_TAG = v0.28.15
DOCKER_TAG := $(FLOW_GO_TAG)

# default values that callers can override when calling target
ACCESS = 1
COLLECTION = 6
VALID_COLLECTION := $(shell test $(COLLECTION) -ge 6; echo $$?)
CONSENSUS = 2
VALID_CONSENSUS := $(shell test $(CONSENSUS) -ge 2; echo $$?)
EXECUTION = 2
VALID_EXECUTION := $(shell test $(EXECUTION) -ge 2; echo $$?)
VERIFICATION = 1

validate:
ifeq ($(strip $(VALID_EXECUTION)), 1)
	# multiple execution nodes are required to prevent seals being generated in case of execution forking.
	$(error Number of Execution nodes should be no less than 2)
else ifeq ($(strip $(VALID_CONSENSUS)), 1)
	$(error Number of Consensus nodes should be no less than 2)
else ifeq ($(strip $(VALID_COLLECTION)), 1)
	$(error Number of Collection nodes should be no less than 6)
else ifeq ($(strip $(NAMESPACE)),)
	$(error Namespace cannot be empty)
endif

# assumes there is a checked out version of flow-go in a "flow-go" sub-folder at this level so that the bootstrap executable
# for the checked out version will be run in the sub folder but the bootstrap folder will be created here (outside of the checked out flow-go in the sub folder)
gen-bootstrap: clone-flow
	cd flow-go/cmd/bootstrap && go run -tags relic . genconfig --address-format "%s%d.${NAMESPACE}:3569" --access $(ACCESS) --collection $(COLLECTION) --consensus $(CONSENSUS) --execution $(EXECUTION) --verification $(VERIFICATION) --weight 1000 -o ./ --config ../../../bootstrap/conf/node-config.json
	cd flow-go/cmd/bootstrap && go run -tags relic . keygen --machine-account --config ../../../bootstrap/conf/node-config.json -o ../../../bootstrap/keys
	echo {} > ./bootstrap/conf/partner-stakes.json
	mkdir ./bootstrap/partner-nodes
	cd flow-go/cmd/bootstrap && go run -tags relic . rootblock  --root-chain bench --root-height 0 --root-parent 0000000000000000000000000000000000000000000000000000000000000000 --config ../../../bootstrap/conf/node-config.json -o ../../../bootstrap/ --fast-kg --partner-dir ../../../bootstrap/partner-nodes --partner-weights ../../../bootstrap/conf/partner-stakes.json --internal-priv-dir ../../../bootstrap/keys/private-root-information
	cd flow-go/cmd/bootstrap && go run -tags relic . finalize --root-commit 0000000000000000000000000000000000000000000000000000000000000000 --service-account-public-key-json "{\"PublicKey\":\"R7MTEDdLclRLrj2MI1hcp4ucgRTpR15PCHAWLM5nks6Y3H7+PGkfZTP2di2jbITooWO4DD1yqaBSAVK8iQ6i0A==\",\"SignAlgo\":2,\"HashAlgo\":1,\"SeqNumber\":0,\"Weight\":1000}" --config ../../../bootstrap/conf/node-config.json -o ../../../bootstrap/ --partner-dir ../../../bootstrap/partner-nodes --partner-weights ../../../bootstrap/conf/partner-stakes.json --collection-clusters 1 --epoch-counter 0 --epoch-length 1600 --epoch-staking-phase-length 50 --epoch-dkg-phase-length 500 --genesis-token-supply="1000000000.0" --protocol-version=0 --internal-priv-dir ../../../bootstrap/keys/private-root-information --dkg-data ../../../bootstrap/private-root-information/root-dkg-data.priv.json --root-block ../../../bootstrap/public-root-information/root-block.json --root-block-votes-dir ../../../bootstrap/public-root-information/root-block-votes/

gen-helm-l1:
	go run automate/cmd/level1/bootstrap.go --data bootstrap/public-root-information/root-protocol-state-snapshot.json --dockerTag $(DOCKER_TAG)

gen-helm-l2:
	go run automate/cmd/level2/template.go --data template-data.json --template automate/templates/helm-values-all-nodes.yml --outPath="./values.yml"

# main target for creating dynamic helm values.yml chart
# runs bootstrap to generate all node info
# runs level 1 automation to read bootstrap data and generate data input for level 2
# runs level 2 automation to generate values.yml based on template and data values from previous step
gen-helm-values: validate gen-bootstrap gen-helm-l1 gen-helm-l2

# main target for deployment
deploy-all: validate gen-helm-values k8s-secrets-create helm-deploy

# main target for cleaning up a deployment
clean-all: validate k8s-delete k8s-delete-secrets clean-bootstrap clean-gen-helm clean-flow

clean-bootstrap:
	rm -rf ./bootstrap

clean-gen-helm:
	rm -f values.yml
	rm -f template-data.json

k8s-secrets-create:
	bash ./create-secrets.sh

helm-deploy:
	helm upgrade --install -f ./values.yml ${NAMESPACE} ./flow --debug --namespace ${NAMESPACE}

k8s-delete:
	helm delete ${NAMESPACE} --namespace ${NAMESPACE}
	kubectl delete pvc -l service=flow --namespace ${NAMESPACE}

k8s-delete-secrets:
	kubectl delete secrets -l service=flow  --namespace ${NAMESPACE}

k8s-expose-locally:
	kubectl port-forward service/access1 9000:9000 --namespace ${NAMESPACE}

k8s-pod-health:
	kubectl get pods

k8s-test-network-accessibility:
	flow blocks get latest --host localhost:9000
	flow accounts create --network benchnet --key  e0ef5e52955e6542287db4528b3e8acc84a2c204eee9609f7c3120d1dac5a11b1bcb39677511db14354aa8c1a0ef62151220d97f015d49a8f0b78b653b570bfd --signer benchnet-account -f ~/flow.json

clone-flow: clean-flow
	# this cloned repo will be used for generating bootstrap info specific to that tag / version
	git clone --depth 1 --branch $(FLOW_GO_TAG) https://github.com/onflow/flow-go.git

clean-flow:
	rm -rf flow-go
