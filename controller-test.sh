#!/bin/bash
NOCOLOR='\033[0m'
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
LIGHTBLUE='\033[1;34m'
MAGENTA='\033[1;35m'

KIND_NAME=chart-testing

# echo -e "${GREEN}Create aergia kind cluster${NOCOLOR}"
# kind create cluster --image kindest/node:v1.21.1 --name aergia --config test-resources/kind-cluster.yaml
# kubectl cluster-info --context kind-aergia
# kubectl config use-context kind-aergia

tear_down () {
  echo -e "${GREEN}============= TEAR DOWN =============${NOCOLOR}"
  kind delete cluster --name ${KIND_NAME}
}

echo -e "${GREEN}Install custom backend handler${NOCOLOR}"
kubectl apply -f test-resources/aergia-backend.yaml
NUM_PODS=$(kubectl -n ingress-nginx get pods | grep -ow "Running"| wc -l |  tr  -d " ")
if [ $NUM_PODS -ne 1 ]; then
    echo -e "${GREEN}Install ingress-nginx${NOCOLOR}"
    helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
    kubectl create namespace ingress-nginx
    ## Pinning to 3.36.0 whilst https://github.com/kubernetes/ingress-nginx/releases/tag/controller-v1.0.0 is an issue
    helm upgrade --install -n ingress-nginx ingress-nginx ingress-nginx/ingress-nginx -f test-resources/ingress-nginx-values.yaml --version 3.36.0
    kubectl get pods --all-namespaces
    echo -e "${GREEN}Wait for ingress-nginx to become ready${NOCOLOR}"
    sleep 60
else
    echo -e "${GREEN}Ingress-nginx is ready${NOCOLOR}"
fi
echo -e "${GREEN}Install example-nginx app with 0 replicas set${NOCOLOR}"
kubectl apply -f test-resources/example-nginx.yaml
sleep 15
echo -e "${GREEN}Check there are no example-nginx pods${NOCOLOR}"
kubectl -n example-nginx get pods
echo -e "${GREEN}Request example-nginx app (should be 503)${NOCOLOR}"
if curl -s -I -H "Host: aergia.localhost" http://localhost:8090/| grep -q "503 Service Unavailable"; then
    sleep 15
    echo -e "${GREEN}Check there are 3 example-nginx pods${NOCOLOR}"
    kubectl -n example-nginx get pods
    echo -e "${GREEN}Request example-nginx app (should be 200)${NOCOLOR}"
    if curl -s -I -H "Host: aergia.localhost" http://localhost:8090/| grep -q "200 OK"; then
        echo -e "${GREEN}Tear down aergia cluster${NOCOLOR}"
        tear_down
    else
        echo -e "${RED}Curl did not return 200${NOCOLOR}"
        exit 1
    fi
else
    echo -e "${RED}Curl did not return 503${NOCOLOR}"
    tear_down
    exit 1
fi
