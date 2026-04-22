# Changelog

Author: Prisacaru Bogdan-Paul 343 C4 <br>
Asistent: Florin Mihalache (Miercuri 08:00-10:00)

**Distributed Log Aggregator** is a log management system that collects, processes, and stores logs from various sources.

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Test the application on a real cluster with one manager and two worker nodes.

## [1.2.0]

### Added
- Comprehensive unit tests with mocks for all Go microservices (`log-ingestor`, `log-consumer`, `log-generator`).
- Integrated GitHub Actions for automated CI testing and verification.
- Postman collection in `tests/` for automated API and RBAC validation.

### Fixed
- Fixed bug where Viewer accounts could see restricted log levels.
- Fixed dashboard auto-submission issue when loading custom queries.

## [1.1.0]

### Added
- Integrated Kong API Gateway for secure and centralized routing.
- Added Portainer for simplified Docker Swarm stack management.
- Implemented Role-Based Access Control (RBAC) using Keycloak (Admin, Developer, Viewer roles).
- Added `manage_ops.sh` utility script for runtime system control.
- Prometheus metrics integration for all custom Go services.

### Changed
- Divide infrastructure into isolated overlay networks (`public_net`, `stream_net`, `data_net`, `monitoring_net`).
- Upgraded **Log Generator** to support dynamic rate and level weight updates via REST API.

## [1.0.0]

### Added
- Initial release of the Distributed Log Aggregator.
- High-throughput **Log Ingestor** service in Go.
- **Log Consumer** service with Kafka-to-OpenSearch bulk indexing.
- Custom **Log Generator** service in Go.
- Authentication and authorization using Keycloak SSO.
- **Dashboard Service** (FastAPI) for log search, filtering, and export.
- 3-broker Kafka cluster with Zookeeper and Kafka-UI.
- OpenSearch and OpenSearch Dashboards for data storage and analysis.
- Monitoring stack with Prometheus, Grafana, and cAdvisor.
- Support for Docker Swarm deployment.
