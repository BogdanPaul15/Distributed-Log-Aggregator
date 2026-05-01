#!/bin/bash
set -euo pipefail
unset DOCKER_HOST

WORKER_COUNT=2
MANAGER_NAME="swarm-manager"
WORKER_PREFIX="swarm-worker"

docker rm -f $MANAGER_NAME >/dev/null 2>&1 || true
for i in $(seq 1 $WORKER_COUNT); do
    docker rm -f "${WORKER_PREFIX}-${i}" >/dev/null 2>&1 || true
done
docker network rm dind-net >/dev/null 2>&1 || true

docker network create dind-net

echo "Starting $MANAGER_NAME..."
docker run -d \
    --privileged \
    --name "$MANAGER_NAME" \
    --hostname manager \
    --network dind-net \
    -v "$(pwd)":"$(pwd)" \
    -w "$(pwd)" \
    -p 2375:2375 \
    -p 8000:8000 \
    -p 8080:8080 \
    -p 8081:8081 \
    -p 8090:8090 \
    -p 8100:8100 \
    -p 8101:8101 \
    -p 3000:3000 \
    -p 5601:5601 \
    -p 9000:9000 \
    -p 9443:9443 \
    -p 9090:9090 \
    -e DOCKER_TLS_CERTDIR="" \
    docker:27-dind

echo "Starting $WORKER_COUNT worker nodes..."
for i in $(seq 1 $WORKER_COUNT); do
    CONTAINER_NAME="${WORKER_PREFIX}-${i}"
    echo "  Starting $CONTAINER_NAME..."
    docker run -d \
        --privileged \
        --name "$CONTAINER_NAME" \
        --hostname "worker-${i}" \
        --network dind-net \
        -v "$(pwd)":"$(pwd)" \
        -w "$(pwd)" \
        -e DOCKER_TLS_CERTDIR="" \
        docker:27-dind
done

echo "Waiting for Manager Docker daemon to start..."
until docker exec $MANAGER_NAME docker info >/dev/null 2>&1; do sleep 1; done

echo "Initializing Swarm on Manager..."
MANAGER_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $MANAGER_NAME)
docker exec $MANAGER_NAME docker swarm init --advertise-addr $MANAGER_IP
SWARM_TOKEN=$(docker exec $MANAGER_NAME docker swarm join-token worker -q)

echo "Joining workers to Swarm..."
for i in $(seq 1 $WORKER_COUNT); do
    CONTAINER_NAME="${WORKER_PREFIX}-${i}"
    until docker exec $CONTAINER_NAME docker info >/dev/null 2>&1; do sleep 1; done
    docker exec $CONTAINER_NAME docker swarm join --token "$SWARM_TOKEN" "$MANAGER_IP:2377"
    echo "  $CONTAINER_NAME joined!"
done

echo ""
echo "=== Cluster Nodes ==="
docker exec $MANAGER_NAME docker node ls

echo ""
echo "=== Cluster is ready! ==="
echo ""
echo "IMPORTANT: YOU MUST RUN THIS COMMAND FIRST TO USE THE CLUSTER:"
echo "  export DOCKER_HOST=tcp://127.0.0.1:2375"
echo ""
echo "Then you can deploy your stack normally:"
echo "  bash reset_and_start.sh"
