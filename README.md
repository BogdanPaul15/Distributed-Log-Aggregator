# Distributed Log Aggregator

A scalable, microservices-based log aggregation and analytics platform built with **Docker Swarm**, **FastAPI**, **Keycloak**, and **OpenSearch**.

## Architecture

The system consists of the following microservices:

### 1. Dashboard Service
A FastAPI-based frontend and API that provides a user interface for log visualization, filtering, and exporting. It handles authentication via Keycloak.
- **Port**: `8000` (Exposed)
- **Repo Path**: `./dashboard_service`

**API Endpoints:**
- `GET /` - Main dashboard UI. Supports query parameters for filtering: `q` (search), `service`, `level`, `start_time`, `end_time`.
- `GET /export` - Export logs. Requires `admin` or `developer` role.
    - **Params**: `format` (csv/json), plus filter params.
- `GET /login`, `/logout`, `/callback` - OIDC Authentication flow.

### 2. Ingestor Service
A high-throughput Go service dedicated to receiving logs from various sources and pushing them to Kafka.
- **Port**: `8080` (Internal to Swarm network)
- **Repo Path**: `./log-ingestor`
- **Metrics**: `/metrics` (Prometheus)

**API Endpoints:**
- `POST /logs` - Ingest a batch of logs.
    - **Body**: JSON Array of LogEvents.
    ```json
    [
      {
        "service": "payment-service",
        "level": "INFO",
        "message": "Transaction processed",
        "timestamp": "2024-01-01T12:00:00Z"
      }
    ]
    ```

### 3. Log Generator
A utility service written in Go that simulates a distributed system by generating realistic log traffic.
- **Port**: `8081` (Exposed)
- **Repo Path**: `./log-generator`
- **Function**: Continuously sends random logs to the **Ingestor Service**.

**API Endpoints:**
- `POST /rate` - Update log generation rate.
    - **Body**: `{"rate": 100}`
- `POST /weights` - Update log level weights.
    - **Body**: `{"INFO": 50, "ERROR": 10, ...}`
- `GET /metrics` - Prometheus metrics.

### 4. Log Consumer
A Go service that consumes log messages from Kafka and indexes them into OpenSearch.
- **Repo Path**: `./log-consumer`
- **Function**: Reads from `logs` topic, buffers messages, and bulk indexes them into OpenSearch. Calculates lag metrics.

### 5. Infrastructure
- **OpenSearch**: Search and analytics engine (`9200`).
- **Keycloak**: IAM and SSO (`8080`).
- **PostgreSQL**: Database for user metadata (`5432`).
- **Kafka & Zookeeper**: Message queue system.
- **Prometheus & Grafana**: Monitoring and observability stack (`9090`, `3000`).

### 6. Kafka Architecture
The system uses a Kafka topic named `logs` for buffering log events.

- **Topic**: `logs`
- **Partitions**: 4
- **Replication Factor**: 1 (Single broker setup)
- **Partitioning Strategy**:
    - Messages are **Keyed by Log Level** (e.g., `INFO`, `ERROR`).
    - The producer uses a **LeastBytes** balancing strategy to distribute load across partitions while attaching the key.

## Key Features

### 1. Authentication
*   **Single Sign-On (SSO)**: Powered by **Keycloak** (OpenID Connect).

### 2. Role-Based Access Control (RBAC)
Secure access management with distinct roles:

| Feature | Admin | Developer | Viewer |
| :--- | :---: | :---: | :---: |
| **Login via SSO** | ✅ | ✅ | ✅ |
| **Search & Filter** | ✅ | ✅ | ✅ |
| **Time Window** | Unlimited | Unlimited | Last 3 Hours |
| **Log Visibility** | All Levels | All Levels | INFO/WARN Only |
| **Export Data** | ✅ | ✅ | ❌ |

### 3. Advanced Filtering & Search
*   **Full-Text Search**: Search log messages using OpenSearch.
*   **Structured Filters**: Filter by **Service**, **Log Level**, and **Time Range**.
*   **Quick Ranges**: Predefined buttons for "Last 5m", "Last 1h", etc.
*   **Local Time**: Automatic timezone conversion for accurate searching.

### 4. Data Export
*   Export filtered logs to **CSV** or **JSON** formats.
*   Respects active filters.

### 5. Scalable Ingestion
*   Time-based indexing strategy (`app-logs-YYYY.MM.DD`).
*   Buffered Kafka processing.

## Setup & Deployment

1.  **Build the Services**:
    ```bash
    docker compose build
    ```

2.  **Deploy the Stack**:
    ```bash
    docker stack deploy -c docker-compose.yml log_stack
    ```

3.  **Access the Dashboard**:
    Open [http://localhost:8000](http://localhost:8000) in your browser.

4.  **Monitoring**:
    - **Grafana**: [http://localhost:3000](http://localhost:3000) (User: `admin`, Pass: `admin`)
    - **OpenSearch Dashboards**: [http://localhost:5601](http://localhost:5601)

## Default Credentials

**Application Users (Log Realm):**

| Role | Username | Password |
| :--- | :--- | :--- |
| **Admin** | `admin` | `admin` |
| **Developer** | `developer` | `developer` |
| **Viewer** | `viewer` | `viewer` |

**Keycloak Administration Console:**
*   **URL**: [http://localhost:8080](http://localhost:8080)
*   **Username**: `admin`
*   **Password**: `admin`

## Testing

To scale the log generator:
```bash
docker service scale log_stack_log-generator=1 # Start
docker service scale log_stack_log-generator=0 # Stop
```