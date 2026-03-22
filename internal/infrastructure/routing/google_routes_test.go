package routing

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"smallworld/internal/domain"
)

func TestGoogleRoutesProviderRoute(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.Header.Get("X-Goog-Api-Key"); got != "test-key" {
				t.Fatalf("api key header = %q", got)
			}
			if got := r.Header.Get("X-Goog-FieldMask"); got != "routes.distanceMeters,routes.duration,routes.polyline.encodedPolyline" {
				t.Fatalf("field mask = %q", got)
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			bodyString := string(body)
			for _, needle := range []string{
				`"travelMode":"DRIVE"`,
				`"routingPreference":"TRAFFIC_AWARE"`,
				`"polylineQuality":"OVERVIEW"`,
			} {
				if !strings.Contains(bodyString, needle) {
					t.Fatalf("request body missing %s: %s", needle, bodyString)
				}
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewBufferString(
					`{"routes":[{"distanceMeters":1823,"duration":"305s","polyline":{"encodedPolyline":"abc123"}}]}`,
				)),
				Header: make(http.Header),
			}, nil
		}),
	}

	provider, err := NewGoogleRoutesProvider(GoogleRoutesConfig{
		APIKey:     "test-key",
		BaseURL:    "https://example.test/computeRoutes",
		HTTPClient: client,
	})
	if err != nil {
		t.Fatalf("NewGoogleRoutesProvider() error = %v", err)
	}

	route, err := provider.Route(context.Background(), domain.Location{Lat: 45.46, Lng: 9.19}, domain.Location{Lat: 45.48, Lng: 9.22})
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if route.DistanceMeters != 1823 {
		t.Fatalf("DistanceMeters = %d", route.DistanceMeters)
	}
	if route.DurationSeconds != 305 {
		t.Fatalf("DurationSeconds = %d", route.DurationSeconds)
	}
	if route.Polyline != "abc123" {
		t.Fatalf("Polyline = %q", route.Polyline)
	}
}

func TestGoogleRoutesProviderWalkingETA(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.Header.Get("X-Goog-FieldMask"); got != "routes.distanceMeters,routes.duration" {
				t.Fatalf("field mask = %q", got)
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			bodyString := string(body)
			if !strings.Contains(bodyString, `"travelMode":"WALK"`) {
				t.Fatalf("request body missing walking travel mode: %s", bodyString)
			}
			if strings.Contains(bodyString, `"routingPreference"`) {
				t.Fatalf("routingPreference should be omitted for walking: %s", bodyString)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"routes":[{"distanceMeters":640,"duration":"480s"}]}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	provider, err := NewGoogleRoutesProvider(GoogleRoutesConfig{
		APIKey:     "test-key",
		BaseURL:    "https://example.test/computeRoutes",
		HTTPClient: client,
	})
	if err != nil {
		t.Fatalf("NewGoogleRoutesProvider() error = %v", err)
	}

	durationSeconds, distanceMeters, err := provider.WalkingETASeconds(context.Background(), domain.Location{Lat: 45.46, Lng: 9.19}, domain.Location{Lat: 45.47, Lng: 9.20})
	if err != nil {
		t.Fatalf("WalkingETASeconds() error = %v", err)
	}
	if durationSeconds != 480 {
		t.Fatalf("durationSeconds = %d", durationSeconds)
	}
	if distanceMeters != 640 {
		t.Fatalf("distanceMeters = %d", distanceMeters)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
