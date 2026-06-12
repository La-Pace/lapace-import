package core

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReconstructTimestamps_Basic(t *testing.T) {
	ts := ReconstructTimestamps(5, 60, 1000.0)
	assert.Len(t, ts, 5)
	assert.InDelta(t, 1000.0, ts[0], 1e-9)
	assert.InDelta(t, 1000.0+1.0/60.0, ts[1], 1e-9)
	assert.InDelta(t, 1000.0+4.0/60.0, ts[4], 1e-9)
}

func TestReconstructTimestamps_ZeroRows(t *testing.T) {
	ts := ReconstructTimestamps(0, 60, 1000.0)
	assert.Len(t, ts, 0)
}

func TestReconstructTimestamps_ZeroOffset(t *testing.T) {
	ts := ReconstructTimestamps(3, 100, 0.0)
	assert.InDelta(t, 0.0, ts[0], 1e-9)
	assert.InDelta(t, 0.01, ts[1], 1e-9)
	assert.InDelta(t, 0.02, ts[2], 1e-9)
}

func TestReconstructTimestamps_Monotonic(t *testing.T) {
	ts := ReconstructTimestamps(1000, 60, 1715000000.0)
	for i := 1; i < len(ts); i++ {
		assert.True(t, ts[i] > ts[i-1], "ts[%d] <= ts[%d]: %f <= %f", i, i-1, ts[i], ts[i-1])
	}
}

func TestReconstructTimestamps_Precision(t *testing.T) {
	ts := ReconstructTimestamps(360, 360, 0.0)
	expected := 1.0 / 360.0
	assert.True(t, math.Abs(ts[1]-expected) < 1e-12, "expected %f, got %f", expected, ts[1])
}
