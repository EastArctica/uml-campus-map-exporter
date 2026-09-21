# UML Campus Map Exporter

Prometheus exporter for UMass Lowell's public campus-map APIs. It exposes parking capacity and availability, active shuttle lines and stops, vehicle positions, transit service-update state, and campus-map points of interest.

## Deployment

Run the published container with Docker Compose:

```yaml
services:
  uml-campus-map-exporter:
    image: ghcr.io/eastarctica/uml-campus-map-exporter:latest
    restart: unless-stopped
    ports:
      - "9828:9828"
    read_only: true
    security_opt:
      - no-new-privileges:true
```

The exporter listens on port `9828` and exposes:

- `/metrics`: Prometheus metrics
- `/-/healthy`: process health check

The container has no persistent state and requires outbound HTTPS access to `www.uml.edu`.

## Prometheus

Add the exporter to Prometheus using its reachable hostname or IP address:

```yaml
scrape_configs:
  - job_name: uml-campus-map
    scrape_interval: 30s
    static_configs:
      - targets:
          - uml-campus-map-exporter:9828
```

Each Prometheus scrape requests UML's live upstream APIs. The exporter fetches the five upstream endpoints concurrently, with a per-request timeout of 10 seconds by default. Monitor `uml_upstream_up` and `uml_upstream_request_duration_seconds` to distinguish upstream failures from missing UML data.

## Configuration

Configuration is provided through command-line flags.

| Flag | Default | Description |
| --- | --- | --- |
| `-web.listen-address` | `:9828` | Address on which to serve metrics and health checks. |
| `-uml.base-url` | `https://www.uml.edu` | Base URL for UML's public APIs. Useful for a proxy or test fixture. |
| `-uml.timeout` | `10s` | Timeout applied to each upstream API request. |

## Metrics

| Metric | Description |
| --- | --- |
| `uml_parking_spaces_total` | Reported parking capacity by availability record, lot, user type, and status. |
| `uml_parking_spaces_available` | Available parking spaces for each reported availability record. |
| `uml_transit_active_lines` | Number of active shuttle lines. |
| `uml_transit_line_info` | Active line metadata. |
| `uml_transit_line_stop_info` | Stops served by each active line. |
| `uml_transit_active_vehicles` | Number of currently active vehicles. |
| `uml_transit_vehicle_info` | Active vehicle metadata. |
| `uml_transit_vehicle_latitude` | Vehicle latitude. |
| `uml_transit_vehicle_longitude` | Vehicle longitude. |
| `uml_transit_vehicle_speed` | Reported vehicle speed. |
| `uml_transit_vehicle_heading_degrees` | Reported vehicle heading when supplied by UML. |
| `uml_transit_status_updates` | Number of currently published transit service updates. |
| `uml_transit_latest_status_update_timestamp_seconds` | Latest published transit service-update timestamp. |
| `uml_map_point_of_interest_info` | Campus-map point-of-interest metadata. |
| `uml_map_point_of_interest_latitude` | Point-of-interest latitude. |
| `uml_map_point_of_interest_longitude` | Point-of-interest longitude. |
| `uml_upstream_up` | Whether an individual UML upstream API request succeeded. |
| `uml_upstream_request_duration_seconds` | Duration of an individual UML upstream API request. |

The public map API does not supply a structured building-capacity field. The exporter does not derive capacity from unstructured building descriptions.

## Local Development

The root `compose.yaml` builds the local checkout rather than pulling GHCR:

```sh
docker compose up --build
```

Run checks locally with:

```sh
go test ./...
go vet ./...
```
