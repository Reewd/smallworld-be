package service

import (
	"context"

	"smallworld/internal/domain"
	"smallworld/internal/ports"
)

type VerificationService struct {
	verifications ports.VerificationRepository
}

func NewVerificationService(verifications ports.VerificationRepository) *VerificationService {
	return &VerificationService{verifications: verifications}
}

func (s *VerificationService) FindByUserID(ctx context.Context, userID string) (domain.IdentityVerification, error) {
	return s.verifications.FindByUserID(ctx, userID)
}
