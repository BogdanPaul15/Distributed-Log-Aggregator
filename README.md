# Distributed Log Aggregator

A scalable, microservices-based log aggregation and analytics platform built with **Docker Swarm**, **FastAPI**, **Keycloak**, and **OpenSearch**.

## Architecture

The system consists of the following microservices:

*   **Dashboard Service**: A FastAPI-based frontend and API that provides a user interface for log visualization, filtering, and exporting. It handles authentication via Keycloak.
*   **Ingestor Service**: A lightweight service dedicated to receiving logs from various sources and indexing them into OpenSearch.
*   **Log Generator**: A utility service that generates mock log traffic to simulate a real-world distributed system.
*   **OpenSearch**: The core search and analytics engine used to store and query logs.
*   **Keycloak**: Handles Identity and Access Management (IAM) and Single Sign-On (SSO).
*   **PostgreSQL**: Stores user profile data and application-specific metadata.

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
*   **Pagination**: Efficiently browse through logs.

### 4. Data Export
*   Export filtered logs to **CSV** or **JSON** formats.
*   Respects active filters (e.g., if you filter for "ERROR", the export will only contain errors).

### 5. Scalable Ingestion
*   Time-based indexing strategy (`app-logs-YYYY.MM.DD`) for efficient storage management.

## Setup & Deployment

### Prerequisites
*   Docker & Docker Compose
*   Docker Swarm initialized (`docker swarm init`)

### Deployment
1.  **Build the Services**:
    ```bash
    docker compose build
    ```

2.  **Deploy the Stack**:
    ```bash
    docker stack deploy -c docker-compose.yml log_stack
    ```

2.  **Access the Dashboard**:
    Open [http://localhost:8000](http://localhost:8000) in your browser.

3.  **Access Keycloak Console**:
    Open [http://localhost:8080](http://localhost:8080).

### Default Credentials

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

To generate traffic, ensure the log generator is running:
```bash
docker service scale log_stack_log-generator=1
```
To stop generation:
```bash
docker service scale log_stack_log-generator=0
```