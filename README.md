# UML Campus Map Exporter

Prometheus exporter for UMass Lowell's public campus-map APIs. It exports parking capacity and availability, active shuttle lines and stops, vehicle positions, service-update timestamps, and all map points of interest.

```sh
go run .
```

## Docker

```sh
docker build -t uml-campus-map-exporter .
docker run --rm -p 9828:9828 uml-campus-map-exporter
```

Pass exporter flags after the image name when needed:

```sh
docker run --rm -p 9828:9828 uml-campus-map-exporter -uml.timeout=20s
```

Metrics are available at `http://localhost:9828/metrics`. The exporter requests UML live data for every Prometheus scrape. Use `-uml.base-url` to point tests or a proxy at another API host, and `-uml.timeout` to tune upstream request timeouts.

Key metrics include `uml_parking_spaces_total`, `uml_parking_spaces_available`, `uml_transit_active_vehicles`, `uml_transit_vehicle_latitude`, `uml_transit_vehicle_longitude`, `uml_transit_vehicle_speed`, and `uml_map_point_of_interest_info`.
