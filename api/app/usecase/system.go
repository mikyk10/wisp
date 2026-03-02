package usecase

import "github.com/mikyk10/wisp/app/domain/repository"

type SystemUsecase interface {
	Prune() error
}

type systemUsecase struct {
	sysRepo repository.SystemRepository
}

func NewSystemUsecase(sysRepo repository.SystemRepository) SystemUsecase {
	return &systemUsecase{
		sysRepo: sysRepo,
	}
}

// Prune deletes all the tables and recreates them
func (s *systemUsecase) Prune() error {
	return s.sysRepo.DropAndRecreate()
}
