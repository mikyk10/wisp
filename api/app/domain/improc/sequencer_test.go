package improc_test

import (
	"context"
	"image"
	"testing"

	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/stretchr/testify/assert"
)

type mockProcessor struct {
	callCount int
	order     int
	lastOrder *int
}

func (m *mockProcessor) Apply(ctx context.Context, src image.Image, meta *model.ImgMeta) (image.Image, *model.ImgMeta) {
	m.callCount++
	*m.lastOrder++
	m.order = *m.lastOrder
	return src, meta
}

func TestSequencer_Push(t *testing.T) {
	seq := improc.NewSequencer()
	lastOrder := 0

	p1 := &mockProcessor{lastOrder: &lastOrder}
	p2 := &mockProcessor{lastOrder: &lastOrder}

	seq.Push(p1)
	seq.Push(p2)

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	meta := &model.ImgMeta{}

	_, _ = seq.Apply(context.Background(), img, meta)

	assert.Equal(t, 1, p1.callCount)
	assert.Equal(t, 1, p1.order)
	assert.Equal(t, 1, p2.callCount)
	assert.Equal(t, 2, p2.order)
}

func TestSequencer_Pop(t *testing.T) {
	seq := improc.NewSequencer()
	lastOrder := 0

	p1 := &mockProcessor{lastOrder: &lastOrder}
	p2 := &mockProcessor{lastOrder: &lastOrder}

	seq.Push(p1)
	seq.Push(p2)
	seq.Pop()

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	meta := &model.ImgMeta{}

	_, _ = seq.Apply(context.Background(), img, meta)

	assert.Equal(t, 1, p1.callCount)
	assert.Equal(t, 0, p2.callCount)
}

func TestSequencer_Prepend(t *testing.T) {
	seq := improc.NewSequencer()
	lastOrder := 0

	p1 := &mockProcessor{lastOrder: &lastOrder}
	p2 := &mockProcessor{lastOrder: &lastOrder}

	seq.Push(p1)
	seq.Prepend(p2)

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	meta := &model.ImgMeta{}

	_, _ = seq.Apply(context.Background(), img, meta)

	// p2 should execute first (prepended)
	assert.Equal(t, 1, p2.order)
	assert.Equal(t, 2, p1.order)
}

func TestSequencer_Shift(t *testing.T) {
	seq := improc.NewSequencer()
	lastOrder := 0

	p1 := &mockProcessor{lastOrder: &lastOrder}
	p2 := &mockProcessor{lastOrder: &lastOrder}

	seq.Push(p1)
	seq.Push(p2)
	seq.Shift()

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	meta := &model.ImgMeta{}

	_, _ = seq.Apply(context.Background(), img, meta)

	assert.Equal(t, 0, p1.callCount)
	assert.Equal(t, 1, p2.callCount)
}

func TestSequencerGroup_Basic(t *testing.T) {
	group := improc.NewSequencerGroup()
	lastOrder := 0

	seq1 := improc.NewSequencer()
	seq2 := improc.NewSequencer()

	p1 := &mockProcessor{lastOrder: &lastOrder}
	p2 := &mockProcessor{lastOrder: &lastOrder}

	seq1.Push(p1)
	seq2.Push(p2)

	group.Push(seq1)
	group.Push(seq2)

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	meta := &model.ImgMeta{}

	_, _ = group.Apply(context.Background(), img, meta)

	assert.Equal(t, 1, p1.order)
	assert.Equal(t, 2, p2.order)
}

func TestSequencerGroup_Pop(t *testing.T) {
	group := improc.NewSequencerGroup()
	lastOrder := 0

	seq1 := improc.NewSequencer()
	seq2 := improc.NewSequencer()

	p1 := &mockProcessor{lastOrder: &lastOrder}
	p2 := &mockProcessor{lastOrder: &lastOrder}

	seq1.Push(p1)
	seq2.Push(p2)

	group.Push(seq1)
	group.Push(seq2)
	group.Pop()

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	meta := &model.ImgMeta{}

	_, _ = group.Apply(context.Background(), img, meta)

	// Only p1 should execute after pop
	assert.Equal(t, 1, p1.callCount)
	assert.Equal(t, 0, p2.callCount)
}

func TestSequencerGroup_Prepend(t *testing.T) {
	group := improc.NewSequencerGroup()
	lastOrder := 0

	seq1 := improc.NewSequencer()
	seq2 := improc.NewSequencer()

	p1 := &mockProcessor{lastOrder: &lastOrder}
	p2 := &mockProcessor{lastOrder: &lastOrder}

	seq1.Push(p1)
	seq2.Push(p2)

	group.Push(seq1)
	group.Prepend(seq2)

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	meta := &model.ImgMeta{}

	_, _ = group.Apply(context.Background(), img, meta)

	// p2 should execute first (prepended)
	assert.Equal(t, 1, p2.order)
	assert.Equal(t, 2, p1.order)
}

func TestSequencerGroup_Shift(t *testing.T) {
	group := improc.NewSequencerGroup()
	lastOrder := 0

	seq1 := improc.NewSequencer()
	seq2 := improc.NewSequencer()

	p1 := &mockProcessor{lastOrder: &lastOrder}
	p2 := &mockProcessor{lastOrder: &lastOrder}

	seq1.Push(p1)
	seq2.Push(p2)

	group.Push(seq1)
	group.Push(seq2)
	group.Shift()

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	meta := &model.ImgMeta{}

	_, _ = group.Apply(context.Background(), img, meta)

	// Only p2 should execute after shift
	assert.Equal(t, 0, p1.callCount)
	assert.Equal(t, 1, p2.callCount)
}
