package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
)

func TestCollectorExportsUpstreamData(t *testing.T) {
	responses := map[string]string{
		"/api/v2/UCAPS/Parking/Spaces": `{"data":[{"Id":"space-1","ParkingUser":"Student","Total":100,"Available":25,"Status":"Open"}]}`,
		"/api/transit/shuttle/v1.0/lines/active": `[{
			"id":"blue","name":"Blue Line","color":"#0067b1","stops":[{"stopId":"north","name":"North Campus","code":"NC","isOnCallOnly":false}]
		}]`,
		"/api/transit/shuttle/v1.0/vehicles/active": `[{
			"id":"bus-1","lineId":"blue","name":"Shuttle 1","location":{"coordinates":{"latitude":42.6,"longitude":-71.3},"heading":180,"speed":12}
		}]`,
		"/api/transit/shuttle/v1.0/status_updates": `[{"timestamp":"2026-09-21T14:17:00-04:00"}]`,
		"/api/V2/Campus/Maps/PointsOfInterest": `{"data":{"PointsOfInterest":[{
			"Id":"lot-1","TypeKey":"parkinglot","Campus":"North","Name":"North Garage","Latitude":42.65,"Longitude":-71.32,
			"Properties":{"availableparkingid":{"Value":"space-1"},"parkinglotcode":{"Value":["NG"]}}
		}]}}`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := responses[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(NewCollector(server.URL, time.Second))
	metrics, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	var output strings.Builder
	encoder := expfmt.NewEncoder(&output, expfmt.NewFormat(expfmt.TypeTextPlain))
	for _, metric := range metrics {
		if err := encoder.Encode(metric); err != nil {
			t.Fatalf("encode metric: %v", err)
		}
	}
	text := output.String()
	for _, want := range []string{
		`uml_parking_spaces_total{id="space-1",lot="North Garage",status="Open",user="Student"} 100`,
		`uml_parking_spaces_available{id="space-1",lot="North Garage",status="Open",user="Student"} 25`,
		`uml_transit_active_vehicles 1`,
		`uml_transit_vehicle_latitude{id="bus-1",line_id="blue"} 42.6`,
		`uml_map_point_of_interest_info{campus="North",id="lot-1",name="North Garage",type="parkinglot"} 1`,
		`uml_upstream_up{endpoint="points_of_interest"} 1`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("metric output missing %q:\n%s", want, text)
		}
	}
}

func TestStringOrStrings(t *testing.T) {
	var scalar stringOrStrings
	if err := scalar.UnmarshalJSON([]byte(`"one"`)); err != nil {
		t.Fatal(err)
	}
	var list stringOrStrings
	if err := list.UnmarshalJSON([]byte(`["one", "two"]`)); err != nil {
		t.Fatal(err)
	}
	if scalar.String() != "one" || list.String() != "one,two" {
		t.Fatalf("unexpected values %q and %q", scalar, list)
	}
}
