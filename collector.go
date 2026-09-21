package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "uml"

type Collector struct {
	baseURL *url.URL
	client  *http.Client
	desc    map[string]*prometheus.Desc
}

func NewCollector(base string, timeout time.Duration) *Collector {
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		panic(fmt.Sprintf("invalid UML base URL %q", base))
	}
	return &Collector{
		baseURL: u,
		client:  &http.Client{Timeout: timeout},
		desc: map[string]*prometheus.Desc{
			"up":                prometheus.NewDesc(prometheus.BuildFQName(namespace, "upstream", "up"), "Whether the most recent request to an UML API endpoint succeeded.", []string{"endpoint"}, nil),
			"duration":          prometheus.NewDesc(prometheus.BuildFQName(namespace, "upstream", "request_duration_seconds"), "Time spent requesting an UML API endpoint.", []string{"endpoint"}, nil),
			"parking_total":     prometheus.NewDesc(prometheus.BuildFQName(namespace, "parking", "spaces_total"), "Total parking spaces reported by UML.", []string{"id", "lot", "user", "status"}, nil),
			"parking_available": prometheus.NewDesc(prometheus.BuildFQName(namespace, "parking", "spaces_available"), "Available parking spaces reported by UML.", []string{"id", "lot", "user", "status"}, nil),
			"lines":             prometheus.NewDesc(prometheus.BuildFQName(namespace, "transit", "active_lines"), "Number of active shuttle lines.", nil, nil),
			"line":              prometheus.NewDesc(prometheus.BuildFQName(namespace, "transit", "line_info"), "Information about an active shuttle line.", []string{"id", "name", "color"}, nil),
			"stop":              prometheus.NewDesc(prometheus.BuildFQName(namespace, "transit", "line_stop_info"), "A stop served by an active shuttle line.", []string{"line_id", "stop_id", "name", "code", "on_call_only"}, nil),
			"vehicles":          prometheus.NewDesc(prometheus.BuildFQName(namespace, "transit", "active_vehicles"), "Number of active shuttle vehicles.", nil, nil),
			"vehicle":           prometheus.NewDesc(prometheus.BuildFQName(namespace, "transit", "vehicle_info"), "Information about an active shuttle vehicle.", []string{"id", "name", "line_id"}, nil),
			"latitude":          prometheus.NewDesc(prometheus.BuildFQName(namespace, "transit", "vehicle_latitude"), "Latitude of an active shuttle vehicle.", []string{"id", "line_id"}, nil),
			"longitude":         prometheus.NewDesc(prometheus.BuildFQName(namespace, "transit", "vehicle_longitude"), "Longitude of an active shuttle vehicle.", []string{"id", "line_id"}, nil),
			"speed":             prometheus.NewDesc(prometheus.BuildFQName(namespace, "transit", "vehicle_speed"), "Speed reported for an active shuttle vehicle.", []string{"id", "line_id"}, nil),
			"heading":           prometheus.NewDesc(prometheus.BuildFQName(namespace, "transit", "vehicle_heading_degrees"), "Heading reported for an active shuttle vehicle.", []string{"id", "line_id"}, nil),
			"updates":           prometheus.NewDesc(prometheus.BuildFQName(namespace, "transit", "status_updates"), "Number of transit service status updates currently published.", nil, nil),
			"latest_update":     prometheus.NewDesc(prometheus.BuildFQName(namespace, "transit", "latest_status_update_timestamp_seconds"), "Timestamp of the latest transit service status update.", nil, nil),
			"poi":               prometheus.NewDesc(prometheus.BuildFQName(namespace, "map", "point_of_interest_info"), "A point of interest in the UML campus map.", []string{"id", "name", "type", "campus"}, nil),
			"poi_latitude":      prometheus.NewDesc(prometheus.BuildFQName(namespace, "map", "point_of_interest_latitude"), "Latitude of a UML map point of interest.", []string{"id"}, nil),
			"poi_longitude":     prometheus.NewDesc(prometheus.BuildFQName(namespace, "map", "point_of_interest_longitude"), "Longitude of a UML map point of interest.", []string{"id"}, nil),
		},
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	var parking parkingResponse
	var lines []line
	var vehicles []vehicle
	var updates []statusUpdate
	var pois poiResponse
	results := []struct {
		endpoint string
		target   any
	}{
		{"parking", &parking},
		{"lines", &lines},
		{"vehicles", &vehicles},
		{"status_updates", &updates},
		{"points_of_interest", &pois},
	}
	var wg sync.WaitGroup
	for i := range results {
		result := &results[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			err := c.get(endpointPath(result.endpoint), result.target)
			up := 0.0
			if err == nil {
				up = 1
			}
			ch <- prometheus.MustNewConstMetric(c.desc["up"], prometheus.GaugeValue, up, result.endpoint)
			ch <- prometheus.MustNewConstMetric(c.desc["duration"], prometheus.GaugeValue, time.Since(start).Seconds(), result.endpoint)
		}()
	}
	wg.Wait()

	lotNames := parkingLotNames(pois)
	for _, p := range parking.Data {
		lot := lotNames[p.ID]
		ch <- prometheus.MustNewConstMetric(c.desc["parking_total"], prometheus.GaugeValue, p.Total, p.ID, lot, p.ParkingUser, p.Status)
		ch <- prometheus.MustNewConstMetric(c.desc["parking_available"], prometheus.GaugeValue, p.Available, p.ID, lot, p.ParkingUser, p.Status)
	}
	ch <- prometheus.MustNewConstMetric(c.desc["lines"], prometheus.GaugeValue, float64(len(lines)))
	for _, line := range lines {
		ch <- prometheus.MustNewConstMetric(c.desc["line"], prometheus.GaugeValue, 1, line.ID, line.Name, line.Color)
		for _, stop := range line.Stops {
			ch <- prometheus.MustNewConstMetric(c.desc["stop"], prometheus.GaugeValue, 1, line.ID, stop.StopID, stop.Name, stop.Code, fmt.Sprint(stop.IsOnCallOnly))
		}
	}
	ch <- prometheus.MustNewConstMetric(c.desc["vehicles"], prometheus.GaugeValue, float64(len(vehicles)))
	for _, v := range vehicles {
		labels := []string{v.ID, v.LineID}
		ch <- prometheus.MustNewConstMetric(c.desc["vehicle"], prometheus.GaugeValue, 1, v.ID, v.Name, v.LineID)
		ch <- prometheus.MustNewConstMetric(c.desc["latitude"], prometheus.GaugeValue, v.Location.Coordinates.Latitude, labels...)
		ch <- prometheus.MustNewConstMetric(c.desc["longitude"], prometheus.GaugeValue, v.Location.Coordinates.Longitude, labels...)
		ch <- prometheus.MustNewConstMetric(c.desc["speed"], prometheus.GaugeValue, v.Location.Speed, labels...)
		if v.Location.Heading != nil {
			ch <- prometheus.MustNewConstMetric(c.desc["heading"], prometheus.GaugeValue, *v.Location.Heading, labels...)
		}
	}
	ch <- prometheus.MustNewConstMetric(c.desc["updates"], prometheus.GaugeValue, float64(len(updates)))
	var newest time.Time
	for _, update := range updates {
		if update.Timestamp.After(newest) {
			newest = update.Timestamp
		}
	}
	if !newest.IsZero() {
		ch <- prometheus.MustNewConstMetric(c.desc["latest_update"], prometheus.GaugeValue, float64(newest.Unix()))
	}
	for _, poi := range pois.Data.PointsOfInterest {
		ch <- prometheus.MustNewConstMetric(c.desc["poi"], prometheus.GaugeValue, 1, poi.ID, poi.Name, poi.TypeKey, poi.Campus)
		ch <- prometheus.MustNewConstMetric(c.desc["poi_latitude"], prometheus.GaugeValue, poi.Latitude, poi.ID)
		ch <- prometheus.MustNewConstMetric(c.desc["poi_longitude"], prometheus.GaugeValue, poi.Longitude, poi.ID)
	}
}

func endpointPath(endpoint string) string {
	return map[string]string{
		"parking":            "/api/v2/UCAPS/Parking/Spaces",
		"lines":              "/api/transit/shuttle/v1.0/lines/active",
		"vehicles":           "/api/transit/shuttle/v1.0/vehicles/active",
		"status_updates":     "/api/transit/shuttle/v1.0/status_updates",
		"points_of_interest": "/api/V2/Campus/Maps/PointsOfInterest",
	}[endpoint]
}

func (c *Collector) get(path string, target any) error {
	u := c.baseURL.ResolveReference(&url.URL{Path: path})
	response, err := c.client.Get(u.String())
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("GET %s: %s", path, response.Status)
	}
	return json.NewDecoder(io.LimitReader(response.Body, 10<<20)).Decode(target)
}

func parkingLotNames(response poiResponse) map[string]string {
	names := make(map[string]string)
	for _, poi := range response.Data.PointsOfInterest {
		property, ok := poi.Properties["availableparkingid"]
		if !ok {
			continue
		}
		for _, id := range strings.Split(property.Value.String(), ",") {
			names[strings.TrimSpace(id)] = poi.Name
		}
	}
	return names
}

type parkingResponse struct {
	Data []parkingSpace `json:"data"`
}
type parkingSpace struct {
	ID          string  `json:"Id"`
	ParkingUser string  `json:"ParkingUser"`
	Total       float64 `json:"Total"`
	Available   float64 `json:"Available"`
	Status      string  `json:"Status"`
}
type line struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Stops []stop `json:"stops"`
}
type stop struct {
	StopID       string `json:"stopId"`
	Name         string `json:"name"`
	Code         string `json:"code"`
	IsOnCallOnly bool   `json:"isOnCallOnly"`
}
type vehicle struct {
	ID       string          `json:"id"`
	LineID   string          `json:"lineId"`
	Name     string          `json:"name"`
	Location vehicleLocation `json:"location"`
}
type vehicleLocation struct {
	Coordinates coordinates `json:"coordinates"`
	Heading     *float64    `json:"heading"`
	Speed       float64     `json:"speed"`
}
type coordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
type statusUpdate struct {
	Timestamp time.Time `json:"timestamp"`
}
type poiResponse struct {
	Data poiData `json:"data"`
}
type poiData struct {
	PointsOfInterest []poi `json:"PointsOfInterest"`
}
type poi struct {
	ID         string                 `json:"Id"`
	TypeKey    string                 `json:"TypeKey"`
	Campus     string                 `json:"Campus"`
	Name       string                 `json:"Name"`
	Latitude   float64                `json:"Latitude"`
	Longitude  float64                `json:"Longitude"`
	Properties map[string]poiProperty `json:"Properties"`
}
type poiProperty struct {
	Value stringOrStrings `json:"Value"`
}
type stringOrStrings string

func (v *stringOrStrings) UnmarshalJSON(data []byte) error {
	var s string
	if json.Unmarshal(data, &s) == nil {
		*v = stringOrStrings(s)
		return nil
	}
	var values []string
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	*v = stringOrStrings(strings.Join(values, ","))
	return nil
}
func (v stringOrStrings) String() string { return string(v) }
