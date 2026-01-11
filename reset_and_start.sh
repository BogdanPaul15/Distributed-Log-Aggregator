#!/bin/bash
set -e

# 0. Ensure Docker Swarm is initialized
if [ "$(docker info --format '{{.Swarm.LocalNodeState}}')" != "active" ]; then
    echo "⚠️  Docker Swarm is not initialized. Initializing..."
    docker swarm init || echo "Failed to initialize swarm. If multiple IPs, run 'docker swarm init --advertise-addr <IP>'"
fi

# 1. Remove the existing stack
echo "🛑 Stopping and removing the stack..."
docker stack rm log_stack || true

# 2. Wait for containers to fully stop (important for volume removal)
echo "⏳ Waiting 15 seconds for containers to stop..."
sleep 15

# 3. Remove persistent volumes (This deletes ALL data)
echo "🧹 Removing persistent volumes (Database & Logs)..."
# Note: Volume names might vary slightly depending on directory name, 
# but usually follow project_volume pattern.
docker volume rm log_stack_db_data log_stack_opensearch_data log_stack_kafka_data log_stack_zookeeper_data log_stack_zookeeper_log || echo "⚠️  Volumes might already be removed or named differently."

# 4. Rebuild the images to ensure latest code is used
echo "🏗️  Building services..."
docker compose build

# 5. Deploy the stack fresh
echo "🚀 Deploying the stack..."
docker stack deploy -c docker-compose.yml log_stack

echo "✅ Deployment complete!"
echo "   - Dashboard: http://localhost:8000"
echo "   - Keycloak:  http://localhost:8080"
echo "   - Grafana:   http://localhost:3000"
echo "   - Kafka UI:  http://localhost:8090"
echo ""
