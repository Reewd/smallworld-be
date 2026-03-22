package routing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/ports"
)

const defaultGoogleRoutesBaseURL = "https://routes.googleapis.com/directions/v2:computeRoutes"

type GoogleRoutesConfig struct {
	APIKey       string
	BaseURL      string
	LanguageCode string
	HTTPClient   *http.Client
}

type GoogleRoutesProvider struct {
	apiKey       string
	baseURL      string
	languageCode string
	httpClient   *http.Client
}

func NewGoogleRoutesProvider(cfg GoogleRoutesConfig) (*GoogleRoutesProvider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("google routes api key is required")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultGoogleRoutesBaseURL
	}

	languageCode := cfg.LanguageCode
	if languageCode == "" {
		languageCode = "en-US"
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	return &GoogleRoutesProvider{
		apiKey:       cfg.APIKey,
		baseURL:      baseURL,
		languageCode: languageCode,
		httpClient:   client,
	}, nil
}

func (p *GoogleRoutesProvider) Route(ctx context.Context, origin, destination domain.Location) (ports.RoutePlan, error) {
	route, err := p.computeRoute(ctx, origin, destination, googleTravelModeDrive, true, "routes.distanceMeters,routes.duration,routes.polyline.encodedPolyline")
	if err != nil {
		return ports.RoutePlan{}, err
	}

	return ports.RoutePlan{
		DistanceMeters:  route.DistanceMeters,
		DurationSeconds: route.DurationSeconds,
		Polyline:        route.Polyline,
	}, nil
}

func (p *GoogleRoutesProvider) WalkingETASeconds(ctx context.Context, origin, destination domain.Location) (int, int, error) {
	route, err := p.computeRoute(ctx, origin, destination, googleTravelModeWalk, false, "routes.distanceMeters,routes.duration")
	if err != nil {
		return 0, 0, err
	}
	return route.DurationSeconds, route.DistanceMeters, nil
}

func (p *GoogleRoutesProvider) DrivingETASeconds(ctx context.Context, origin, destination domain.Location) (int, int, error) {
	route, err := p.computeRoute(ctx, origin, destination, googleTravelModeDrive, false, "routes.distanceMeters,routes.duration")
	if err != nil {
		return 0, 0, err
	}
	return route.DurationSeconds, route.DistanceMeters, nil
}

type googleTravelMode string

const (
	googleTravelModeDrive googleTravelMode = "DRIVE"
	googleTravelModeWalk  googleTravelMode = "WALK"
)

type googleRouteSummary struct {
	DistanceMeters  int
	DurationSeconds int
	Polyline        string
}

func (p *GoogleRoutesProvider) computeRoute(ctx context.Context, origin, destination domain.Location, travelMode googleTravelMode, includePolyline bool, fieldMask string) (googleRouteSummary, error) {
	requestBody := googleComputeRoutesRequest{
		Origin:                   googleWaypoint{Location: googleLocation{LatLng: googleLatLng{Latitude: origin.Lat, Longitude: origin.Lng}}},
		Destination:              googleWaypoint{Location: googleLocation{LatLng: googleLatLng{Latitude: destination.Lat, Longitude: destination.Lng}}},
		TravelMode:               string(travelMode),
		ComputeAlternativeRoutes: false,
		LanguageCode:             p.languageCode,
		Units:                    "METRIC",
	}
	if travelMode == googleTravelModeDrive {
		requestBody.RoutingPreference = "TRAFFIC_AWARE"
		requestBody.RouteModifiers = &googleRouteModifiers{
			AvoidTolls:    false,
			AvoidHighways: false,
			AvoidFerries:  false,
		}
	}
	if includePolyline {
		requestBody.PolylineQuality = "OVERVIEW"
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return googleRouteSummary{}, fmt.Errorf("marshal google routes request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(payload))
	if err != nil {
		return googleRouteSummary{}, fmt.Errorf("build google routes request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", p.apiKey)
	req.Header.Set("X-Goog-FieldMask", fieldMask)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return googleRouteSummary{}, fmt.Errorf("call google routes api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return googleRouteSummary{}, fmt.Errorf("read google routes response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return googleRouteSummary{}, fmt.Errorf("google routes api returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var parsed googleComputeRoutesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return googleRouteSummary{}, fmt.Errorf("decode google routes response: %w", err)
	}
	if len(parsed.Routes) == 0 {
		return googleRouteSummary{}, fmt.Errorf("google routes api returned no routes")
	}

	duration, err := time.ParseDuration(parsed.Routes[0].Duration)
	if err != nil {
		return googleRouteSummary{}, fmt.Errorf("parse google routes duration %q: %w", parsed.Routes[0].Duration, err)
	}

	return googleRouteSummary{
		DistanceMeters:  parsed.Routes[0].DistanceMeters,
		DurationSeconds: int(duration.Seconds()),
		Polyline:        parsed.Routes[0].Polyline.EncodedPolyline,
	}, nil
}

type googleComputeRoutesRequest struct {
	Origin                   googleWaypoint        `json:"origin"`
	Destination              googleWaypoint        `json:"destination"`
	TravelMode               string                `json:"travelMode"`
	RoutingPreference        string                `json:"routingPreference,omitempty"`
	ComputeAlternativeRoutes bool                  `json:"computeAlternativeRoutes"`
	RouteModifiers           *googleRouteModifiers `json:"routeModifiers,omitempty"`
	LanguageCode             string                `json:"languageCode,omitempty"`
	Units                    string                `json:"units,omitempty"`
	PolylineQuality          string                `json:"polylineQuality,omitempty"`
}

type googleWaypoint struct {
	Location googleLocation `json:"location"`
}

type googleLocation struct {
	LatLng googleLatLng `json:"latLng"`
}

type googleLatLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type googleRouteModifiers struct {
	AvoidTolls    bool `json:"avoidTolls"`
	AvoidHighways bool `json:"avoidHighways"`
	AvoidFerries  bool `json:"avoidFerries"`
}

type googleComputeRoutesResponse struct {
	Routes []struct {
		DistanceMeters int    `json:"distanceMeters"`
		Duration       string `json:"duration"`
		Polyline       struct {
			EncodedPolyline string `json:"encodedPolyline"`
		} `json:"polyline"`
	} `json:"routes"`
}
