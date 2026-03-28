package domain

import (
	"encoding/json"
	"errors"
	"time"
)

type Gender string

const (
	GenderUnknown Gender = "unknown"
	GenderFemale  Gender = "female"
	GenderMale    Gender = "male"
	GenderOther   Gender = "other"
)

type VerificationStatus string

const (
	VerificationPending  VerificationStatus = "pending"
	VerificationVerified VerificationStatus = "verified"
	VerificationRejected VerificationStatus = "rejected"
)

type Location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type PreferenceLevel string

const (
	PreferenceLevelLow    PreferenceLevel = "low"
	PreferenceLevelMedium PreferenceLevel = "medium"
	PreferenceLevelBig    PreferenceLevel = "big"
)

type UserPreferences struct {
	WalkToPickup       PreferenceLevel `json:"walk_to_pickup"`
	WalkFromDropoff    PreferenceLevel `json:"walk_from_dropoff"`
	DriverPickupDetour PreferenceLevel `json:"driver_pickup_detour"`
}

func (l PreferenceLevel) IsValid() bool {
	switch l {
	case PreferenceLevelLow, PreferenceLevelMedium, PreferenceLevelBig:
		return true
	default:
		return false
	}
}

func (p UserPreferences) Validate() bool {
	return p.WalkToPickup.IsValid() &&
		p.WalkFromDropoff.IsValid() &&
		p.DriverPickupDetour.IsValid()
}

func (p UserPreferences) MaxWalkToPickupMeters() int {
	switch p.WalkToPickup {
	case PreferenceLevelLow:
		return 200
	case PreferenceLevelBig:
		return 400
	default:
		return 300
	}
}

func (p UserPreferences) MaxWalkFromDropoffMeters() int {
	switch p.WalkFromDropoff {
	case PreferenceLevelLow:
		return 200
	case PreferenceLevelBig:
		return 400
	default:
		return 300
	}
}

func (p UserPreferences) MaxDriverPickupDetourMeters() int {
	switch p.DriverPickupDetour {
	case PreferenceLevelLow:
		return 700
	case PreferenceLevelBig:
		return 1300
	default:
		return 1000
	}
}

func (p *UserPreferences) UnmarshalJSON(data []byte) error {
	var raw struct {
		WalkToPickup       *PreferenceLevel `json:"walk_to_pickup"`
		WalkFromDropoff    *PreferenceLevel `json:"walk_from_dropoff"`
		DriverPickupDetour *PreferenceLevel `json:"driver_pickup_detour"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.WalkToPickup == nil || raw.WalkFromDropoff == nil || raw.DriverPickupDetour == nil {
		return errors.New("all preference fields are required")
	}

	value := UserPreferences{
		WalkToPickup:       *raw.WalkToPickup,
		WalkFromDropoff:    *raw.WalkFromDropoff,
		DriverPickupDetour: *raw.DriverPickupDetour,
	}
	if !value.Validate() {
		return ErrInvalidUserPreferences
	}

	*p = value
	return nil
}

type User struct {
	ID            string          `json:"id"`
	AuthSubject   string          `json:"auth_subject"`
	DisplayName   string          `json:"display_name"`
	AverageRating float64         `json:"average_rating"`
	Preferences   UserPreferences `json:"preferences"`
	CreatedAt     time.Time       `json:"created_at"`
}

type IdentityVerification struct {
	UserID         string             `json:"user_id"`
	Status         VerificationStatus `json:"status"`
	Provider       string             `json:"provider"`
	ProviderRef    string             `json:"provider_ref"`
	VerifiedGender Gender             `json:"verified_gender"`
	VerifiedAt     *time.Time         `json:"verified_at,omitempty"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

type Vehicle struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Make         string    `json:"make"`
	Model        string    `json:"model"`
	Color        string    `json:"color"`
	LicensePlate string    `json:"license_plate"`
	Capacity     int       `json:"capacity"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
}

type DriverSession struct {
	ID                          string             `json:"id"`
	DriverID                    string             `json:"driver_id"`
	VehicleID                   string             `json:"vehicle_id"`
	State                       DriverSessionState `json:"state"`
	Origin                      Location           `json:"origin"`
	Destination                 Location           `json:"destination"`
	CurrentLocation             Location           `json:"current_location"`
	RemainingCapacity           int                `json:"remaining_capacity"`
	MaxDriverPickupDetourMeters int                `json:"max_driver_pickup_detour_meters"`
	RouteDistanceMeters         int                `json:"route_distance_meters"`
	RouteDurationSeconds        int                `json:"route_duration_seconds"`
	RoutePolyline               string             `json:"route_polyline,omitempty"`
	LastHeartbeatAt             time.Time          `json:"last_heartbeat_at"`
	CreatedAt                   time.Time          `json:"created_at"`
	UpdatedAt                   time.Time          `json:"updated_at"`
}

type TripDemand struct {
	ID                       string          `json:"id"`
	RiderID                  string          `json:"rider_id"`
	State                    TripDemandState `json:"state"`
	RequestedOrigin          Location        `json:"requested_origin"`
	RequestedDestination     Location        `json:"requested_destination"`
	MatchedPickup            *Location       `json:"matched_pickup,omitempty"`
	MatchedDropoff           *Location       `json:"matched_dropoff,omitempty"`
	WomenDriversOnly         bool            `json:"women_drivers_only"`
	MaxWalkToPickupMeters    int             `json:"max_walk_to_pickup_meters"`
	MaxWalkFromDropoffMeters int             `json:"max_walk_from_dropoff_meters"`
	IdempotencyKey           string          `json:"idempotency_key"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
}

type RideOffer struct {
	ID               string         `json:"id"`
	DemandID         string         `json:"demand_id"`
	DriverSessionID  string         `json:"driver_session_id"`
	State            RideOfferState `json:"state"`
	DetourMeters     int            `json:"detour_meters"`
	PickupETASeconds int            `json:"pickup_eta_seconds"`
	FareCents        int            `json:"fare_cents"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type RideBooking struct {
	ID                    string           `json:"id"`
	DemandID              string           `json:"demand_id"`
	DriverSessionID       string           `json:"driver_session_id"`
	RiderID               string           `json:"rider_id"`
	DriverID              string           `json:"driver_id"`
	State                 RideBookingState `json:"state"`
	MatchedPickup         Location         `json:"matched_pickup"`
	MatchedDropoff        Location         `json:"matched_dropoff"`
	RiderWalkToPickupM    int              `json:"rider_walk_to_pickup_m"`
	RiderWalkFromDropoffM int              `json:"rider_walk_from_dropoff_m"`
	DriverPickupDetourM   int              `json:"driver_pickup_detour_m"`
	QuotedFareCents       int              `json:"quoted_fare_cents"`
	VehicleLicensePlate   string           `json:"vehicle_license_plate"`
	CreatedAt             time.Time        `json:"created_at"`
	UpdatedAt             time.Time        `json:"updated_at"`
}

type Review struct {
	ID        string    `json:"id"`
	BookingID string    `json:"booking_id"`
	AuthorID  string    `json:"author_id"`
	SubjectID string    `json:"subject_id"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

type MatchCandidate struct {
	Session                    DriverSession `json:"session"`
	Pickup                     Location      `json:"pickup"`
	Dropoff                    Location      `json:"dropoff"`
	Phase                      int           `json:"phase"`
	DriverPickupETASeconds     int           `json:"driver_pickup_eta_seconds"`
	RiderWalkETASeconds        int           `json:"rider_walk_eta_seconds"`
	DriverPickupDetourMeters   int           `json:"driver_pickup_detour_meters"`
	RiderWalkToPickupMeters    int           `json:"rider_walk_to_pickup_meters"`
	RiderWalkFromDropoffMeters int           `json:"rider_walk_from_dropoff_meters"`
	RouteOverlapScore          float64       `json:"route_overlap_score"`
	InCarDistanceMeters        int           `json:"in_car_distance_meters"`
	InCarDurationSeconds       int           `json:"in_car_duration_seconds"`
}
