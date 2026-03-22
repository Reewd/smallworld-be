package service

import (
	"context"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/foundation"
	"smallworld/internal/ports"
)

type VehicleService struct {
	repo ports.VehicleRepository
	idg  foundation.IDGenerator
}

func NewVehicleService(repo ports.VehicleRepository, idg foundation.IDGenerator) *VehicleService {
	return &VehicleService{repo: repo, idg: idg}
}

type CreateVehicleInput struct {
	UserID       string `json:"user_id"`
	Make         string `json:"make"`
	Model        string `json:"model"`
	Color        string `json:"color"`
	LicensePlate string `json:"license_plate"`
	Capacity     int    `json:"capacity"`
}

func (s *VehicleService) Create(ctx context.Context, input CreateVehicleInput) (domain.Vehicle, error) {
	vehicle := domain.Vehicle{
		ID:           s.idg.New("veh"),
		UserID:       input.UserID,
		Make:         input.Make,
		Model:        input.Model,
		Color:        input.Color,
		LicensePlate: input.LicensePlate,
		Capacity:     input.Capacity,
		IsActive:     true,
		CreatedAt:    time.Now().UTC(),
	}
	return vehicle, s.repo.Save(ctx, vehicle)
}

func (s *VehicleService) ListByUserID(ctx context.Context, userID string) ([]domain.Vehicle, error) {
	return s.repo.ListByUserID(ctx, userID)
}
