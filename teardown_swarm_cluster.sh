#!/bin/bash
set -euo pipefail

STACK_NAME="${STACK_NAME:-log_stack}"
MANAGER_NAME="${MANAGER_NAME:-swarm-manager}"
WORKER_PREFIX="${WORKER_PREFIX:-swarm-worker}"
WORKER_COUNT="${WORKER_COUNT:-2}"
DIND_NETWORK="${DIND_NETWORK:-dind-net}"
DIND_DOCKER_HOST="${DIND_DOCKER_HOST:-tcp://127.0.0.1:2375}"
WAIT_SECONDS="${WAIT_SECONDS:-120}"
HOST_COMPOSE_PROJECTS=("${HOST_COMPOSE_PROJECTS:-backend distributed-log-aggregator}")

unset DOCKER_HOST

wait_for_no_services() {
    local waited=0
    while true; do
        local services
        services="$(DOCKER_HOST="$DIND_DOCKER_HOST" docker service ls --filter "label=com.docker.stack.namespace=$STACK_NAME" -q 2>/dev/null || true)"

        if [ -z "$services" ]; then
            echo "Stack services removed."
            return 0
        fi

        if [ "$waited" -ge "$WAIT_SECONDS" ]; then
            echo "Timed out waiting for stack services to be removed (${WAIT_SECONDS}s)."
            return 1
        fi

        sleep 1
        waited=$((waited + 1))
    done
}

remove_prefixed_networks_in_dind() {
    local nets
    nets="$(DOCKER_HOST="$DIND_DOCKER_HOST" docker network ls --format '{{.Name}}' | awk -v stack="$STACK_NAME" '$0 ~ ("^" stack "_") { print }')"

    if [ -n "$nets" ]; then
        echo "$nets" | while IFS= read -r n; do
            [ -z "$n" ] && continue
            DOCKER_HOST="$DIND_DOCKER_HOST" docker network rm "$n" >/dev/null 2>&1 || true
        done
    fi
}

remove_prefixed_volumes_in_dind() {
    local vols
    vols="$(DOCKER_HOST="$DIND_DOCKER_HOST" docker volume ls --format '{{.Name}}' | awk -v stack="$STACK_NAME" '$0 ~ ("^" stack "_") { print }')"

    if [ -n "$vols" ]; then
        echo "$vols" | while IFS= read -r v; do
            [ -z "$v" ] && continue
            DOCKER_HOST="$DIND_DOCKER_HOST" docker volume rm "$v" >/dev/null 2>&1 || true
        done
    fi
}

remove_host_compose_project() {
    local project="$1"
    local containers
    containers="$(docker ps -aq --filter "label=com.docker.compose.project=$project" || true)"

    if [ -n "$containers" ]; then
        echo "Removing host Compose project '$project' containers..."
        docker rm -f $containers >/dev/null 2>&1 || true
    fi

    local networks
    networks="$(docker network ls -q --filter "label=com.docker.compose.project=$project" || true)"
    if [ -n "$networks" ]; then
        docker network rm $networks >/dev/null 2>&1 || true
    fi

    local volumes
    volumes="$(docker volume ls -q --filter "label=com.docker.compose.project=$project" || true)"
    if [ -n "$volumes" ]; then
        docker volume rm $volumes >/dev/null 2>&1 || true
    fi
}

if docker ps -a --format '{{.Names}}' | grep -qx "$MANAGER_NAME"; then
    echo "Cleaning stack resources inside DinD manager daemon ($DIND_DOCKER_HOST)..."

    if DOCKER_HOST="$DIND_DOCKER_HOST" docker info >/dev/null 2>&1; then
        DOCKER_HOST="$DIND_DOCKER_HOST" docker stack rm "$STACK_NAME" >/dev/null 2>&1 || true
        wait_for_no_services || true

        DOCKER_HOST="$DIND_DOCKER_HOST" docker network prune -f >/dev/null 2>&1 || true
        remove_prefixed_networks_in_dind
        remove_prefixed_volumes_in_dind

        DOCKER_HOST="$DIND_DOCKER_HOST" docker swarm leave --force >/dev/null 2>&1 || true
    else
        echo "DinD manager daemon is not reachable via $DIND_DOCKER_HOST, skipping in-daemon cleanup."
    fi
fi

echo "Removing DinD node containers from host daemon..."
containers_to_remove="$(docker ps -a --format '{{.Names}}' | awk -v mgr="$MANAGER_NAME" -v pref="$WORKER_PREFIX" -v count="$WORKER_COUNT" '
    $0 == mgr { print }
    $0 ~ ("^" pref "-[0-9]+$") {
        split($0, parts, "-")
        if (parts[length(parts)] <= count) print
    }
')"

if [ -n "$containers_to_remove" ]; then
    echo "$containers_to_remove" | while IFS= read -r c; do
        [ -z "$c" ] && continue
        docker rm -f "$c" >/dev/null 2>&1 || true
        echo "Removed container: $c"
    done
else
    echo "No DinD node containers found."
fi

echo "Removing DinD network from host daemon..."
docker network rm "$DIND_NETWORK" >/dev/null 2>&1 || true

host_stack_services="$(docker service ls --filter "label=com.docker.stack.namespace=$STACK_NAME" -q 2>/dev/null || true)"
if [ -n "$host_stack_services" ]; then
    echo "Found host-side stack services, removing stack $STACK_NAME..."
    docker stack rm "$STACK_NAME" >/dev/null 2>&1 || true
fi

for project in ${HOST_COMPOSE_PROJECTS[*]}; do
    remove_host_compose_project "$project"
done

host_nets="$(docker network ls --format '{{.Name}}' | awk -v stack="$STACK_NAME" '$0 ~ ("^" stack "_") { print }')"
if [ -n "$host_nets" ]; then
    echo "$host_nets" | while IFS= read -r n; do
        [ -z "$n" ] && continue
        docker network rm "$n" >/dev/null 2>&1 || true
    done
fi

host_vols="$(docker volume ls --format '{{.Name}}' | awk -v stack="$STACK_NAME" '$0 ~ ("^" stack "_") { print }')"
if [ -n "$host_vols" ]; then
    echo "$host_vols" | while IFS= read -r v; do
        [ -z "$v" ] && continue
        docker volume rm "$v" >/dev/null 2>&1 || true
    done
fi

echo "Cleanup complete."
echo "Removed: DinD manager/workers, DinD network, stack-scoped services/networks/volumes (where reachable)."
