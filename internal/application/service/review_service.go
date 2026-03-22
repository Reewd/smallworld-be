package service

import (
	"context"
	"time"

	"smallworld/internal/domain"
	"smallworld/internal/foundation"
	"smallworld/internal/ports"
)

type ReviewService struct {
	reviews  ports.ReviewRepository
	bookings ports.RideBookingRepository
	idg      foundation.IDGenerator
}

func NewReviewService(reviews ports.ReviewRepository, bookings ports.RideBookingRepository, idg foundation.IDGenerator) *ReviewService {
	return &ReviewService{reviews: reviews, bookings: bookings, idg: idg}
}

func (s *ReviewService) Create(ctx context.Context, bookingID, authorID, subjectID string, rating int, comment string) (domain.Review, error) {
	booking, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return domain.Review{}, err
	}
	if booking.State != domain.RideBookingStateCompleted {
		return domain.Review{}, domain.ErrUnauthorized
	}
	review := domain.Review{
		ID:        s.idg.New("review"),
		BookingID: bookingID,
		AuthorID:  authorID,
		SubjectID: subjectID,
		Rating:    rating,
		Comment:   comment,
		CreatedAt: time.Now().UTC(),
	}
	return review, s.reviews.Save(ctx, review)
}

func (s *ReviewService) CreateForActor(ctx context.Context, bookingID, authorID string, rating int, comment string) (domain.Review, error) {
	booking, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return domain.Review{}, err
	}
	if booking.State != domain.RideBookingStateCompleted {
		return domain.Review{}, domain.ErrUnauthorized
	}

	var subjectID string
	switch authorID {
	case booking.RiderID:
		subjectID = booking.DriverID
	case booking.DriverID:
		subjectID = booking.RiderID
	default:
		return domain.Review{}, domain.ErrUnauthorized
	}

	review := domain.Review{
		ID:        s.idg.New("review"),
		BookingID: bookingID,
		AuthorID:  authorID,
		SubjectID: subjectID,
		Rating:    rating,
		Comment:   comment,
		CreatedAt: time.Now().UTC(),
	}
	return review, s.reviews.Save(ctx, review)
}

func (s *ReviewService) ListBySubjectID(ctx context.Context, subjectID string) ([]domain.Review, error) {
	return s.reviews.ListBySubjectID(ctx, subjectID)
}
