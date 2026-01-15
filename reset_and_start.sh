#!/bin/bash
set -e

# 0. Ensure Docker Swarm is initialized
if [ "$(docker info --format '{{.Swarm.LocalNodeState}}')" != "active" ]; then
    echo "Docker Swarm is not initialized. Initializing..."
    docker swarm init || echo "Failed to initialize swarm. If multiple IPs, run 'docker swarm init --advertise-addr <IP>'"
fi

# 1. Remove the existing stack
echo "Stopping and removing the stack..."
docker stack rm log_stack || true

echo "Waiting for stack to be removed..."
until [ -z "$(docker service ls --filter label=com.docker.stack.namespace=log_stack -q)" ]
do
    sleep 1;
done
echo "Stack removed."

# 2.1 Prune unused networks to ensure clean state
echo "Pruning unused networks..."
docker network prune -f

echo "Ensuring stack networks are removed..."
networks=("log_stack_public_net" "log_stack_data_net" "log_stack_stream_net" "log_stack_monitoring_net")
for net in "${networks[@]}"; do
    docker network rm "$net" 2>/dev/null || true
done

echo "Waiting for networks to be fully removed..."
until [ -z "$(docker network ls --filter name=log_stack_ -q)" ]
do
    sleep 1;
done
echo "Networks cleaned."

# 3. Remove persistent volumes (This deletes ALL data)
echo "Removing persistent volumes (Database & Logs)..."
docker volume rm log_stack_db_data log_stack_opensearch_data log_stack_kafka_data_1 log_stack_kafka_data_2 log_stack_kafka_data_3 log_stack_zookeeper_data log_stack_zookeeper_log || echo "Volumes might already be removed or named differently."

# 4. Rebuild the images to ensure latest code is used
echo "Building services..."
docker compose build

# 5. Deploy the stack fresh
echo "Deploying the stack..."
docker stack deploy -c docker-compose.yml log_stack

echo "Deployment complete!"
echo "   - Dashboard: http://localhost:8000"
echo "   - Keycloak:  http://localhost:8080"
echo "   - Grafana:   http://localhost:3000"
echo "   - Kafka UI:  http://localhost:8090"
echo ""
