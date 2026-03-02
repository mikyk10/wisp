package improc

import (
	"context"
	"image"
	"github.com/mikyk10/wisp/app/domain/model"
)

// Sequencer manages and applies image processors in order.
type Sequencer interface {
	Push(improc ImageProcessor)
	Pop()
	Prepend(improc ImageProcessor)
	Shift()
	Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta)
}

func NewSequencer() Sequencer {
	return &sequencer{}
}

type sequencer struct {
	procs []ImageProcessor
}

func (s *sequencer) Push(improc ImageProcessor) {
	s.procs = append(s.procs, improc)
}

func (s *sequencer) Pop() {
	if len(s.procs) > 0 {
		s.procs = s.procs[:len(s.procs)-1]
	}
}

func (s *sequencer) Prepend(improc ImageProcessor) {
	s.procs = append([]ImageProcessor{improc}, s.procs...)
}

func (s *sequencer) Shift() {
	if len(s.procs) > 0 {
		s.procs = s.procs[1:]
	}
}

func (s *sequencer) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {
	img, m := src, meta
	for _, proc := range s.procs {
		img, m = proc.Apply(ctx, img, m)
	}
	return img, m
}

// SequencerGroup manages and applies a group of Sequencers in order.
type SequencerGroup interface {
	Push(grp Sequencer)
	Pop()
	Prepend(grp Sequencer)
	Shift()
	Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta)
}

func NewSequencerGroup() SequencerGroup {
	return &sequencerGroup{}
}

type sequencerGroup struct {
	procs []Sequencer
}

func (s *sequencerGroup) Push(grp Sequencer) {
	s.procs = append(s.procs, grp)
}

func (s *sequencerGroup) Pop() {
	if len(s.procs) > 0 {
		s.procs = s.procs[:len(s.procs)-1]
	}
}

func (s *sequencerGroup) Prepend(grp Sequencer) {
	s.procs = append([]Sequencer{grp}, s.procs...)
}

func (s *sequencerGroup) Shift() {
	if len(s.procs) > 0 {
		s.procs = s.procs[1:]
	}
}

func (s *sequencerGroup) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {
	img, m := src, meta
	for _, seq := range s.procs {
		img, m = seq.Apply(ctx, img, m)
	}
	return img, m
}
