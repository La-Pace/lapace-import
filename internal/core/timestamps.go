package core

// ReconstructTimestamps generates timestamps from a row index, frequency, and
// start epoch. The formula ts[i] = i / frequencyHz + startEpochSeconds is the
// same for both LMU and iRacing.
func ReconstructTimestamps(rowCount int, frequencyHz int, startEpochSeconds float64) []float64 {
	result := make([]float64, rowCount)
	for i := 0; i < rowCount; i++ {
		result[i] = float64(i)/float64(frequencyHz) + startEpochSeconds
	}
	return result
}
