package server

import (
	"math"
)

type Coordinates struct {
	Latitude  float64
	Longitude float64
}

const (
	EarthRadius float64 = 6372797.560856
)

func ValidateCoordinates(longitude float64, latitude float64) bool {
	return longitude <= MAX_LONGITUDE && longitude >= MIN_LONGITUDE && latitude <= MAX_LATITUDE && latitude >= MIN_LATITUDE
}

func spreadInt32ToInt64(v uint32) uint64 {
	result := uint64(v)
	result = (result | (result << 16)) & 0x0000FFFF0000FFFF
	result = (result | (result << 8)) & 0x00FF00FF00FF00FF
	result = (result | (result << 4)) & 0x0F0F0F0F0F0F0F0F
	result = (result | (result << 2)) & 0x3333333333333333
	result = (result | (result << 1)) & 0x5555555555555555
	return result
}

func interleave(x, y uint32) uint64 {
	xSpread := spreadInt32ToInt64(x)
	ySpread := spreadInt32ToInt64(y)
	yShifted := ySpread << 1
	return xSpread | yShifted
}

func Encode(coordinates Coordinates) uint64 {
	// Normalize to the range 0-2^26
	normalizedLatitude := math.Pow(2, 26) * (coordinates.Latitude - MIN_LATITUDE) / LATITUDE_RANGE
	normalizedLongitude := math.Pow(2, 26) * (coordinates.Longitude - MIN_LONGITUDE) / LONGITUDE_RANGE

	// Truncate to integers
	latInt := uint32(normalizedLatitude)
	lonInt := uint32(normalizedLongitude)

	return interleave(latInt, lonInt)
}

func compactInt64ToInt32(v uint64) uint32 {
	result := v & 0x5555555555555555
	result = (result | (result >> 1)) & 0x3333333333333333
	result = (result | (result >> 2)) & 0x0F0F0F0F0F0F0F0F
	result = (result | (result >> 4)) & 0x00FF00FF00FF00FF
	result = (result | (result >> 8)) & 0x0000FFFF0000FFFF
	result = (result | (result >> 16)) & 0x00000000FFFFFFFF
	return uint32(result)
}

func convertGridNumbersToCoordinates(gridLatitudeNumber, gridLongitudeNumber uint32) Coordinates {
	// Calculate the grid boundaries
	gridLatitudeMin := MIN_LATITUDE + LATITUDE_RANGE*(float64(gridLatitudeNumber)/math.Pow(2, 26))
	gridLatitudeMax := MIN_LATITUDE + LATITUDE_RANGE*(float64(gridLatitudeNumber+1)/math.Pow(2, 26))
	gridLongitudeMin := MIN_LONGITUDE + LONGITUDE_RANGE*(float64(gridLongitudeNumber)/math.Pow(2, 26))
	gridLongitudeMax := MIN_LONGITUDE + LONGITUDE_RANGE*(float64(gridLongitudeNumber+1)/math.Pow(2, 26))

	// Calculate the center point of the grid cell
	latitude := (gridLatitudeMin + gridLatitudeMax) / 2
	longitude := (gridLongitudeMin + gridLongitudeMax) / 2

	return Coordinates{Latitude: latitude, Longitude: longitude}
}

func decode(geoCode uint64) Coordinates {
	// Align bits of both latitude and longitude to take even-numbered position
	y := geoCode >> 1
	x := geoCode

	// Compact bits back to 32-bit ints
	gridLatitudeNumber := compactInt64ToInt32(x)
	gridLongitudeNumber := compactInt64ToInt32(y)

	return convertGridNumbersToCoordinates(gridLatitudeNumber, gridLongitudeNumber)
}

type CoordinatedInRadian struct {
	phi    float64
	lambda float64
}

func GetRadian(p Coordinates) CoordinatedInRadian {
	return CoordinatedInRadian{
		phi:    p.Latitude * math.Pi / 180,
		lambda: p.Longitude * math.Pi / 180,
	}
}

func haversine(theta float64) float64 {
	return .5 * (1 - math.Cos(theta))
}

func Distance(p1 CoordinatedInRadian, p2 CoordinatedInRadian) float64 {
	//distance = 2*radius *asin()* sqrt(haversion(delta latitude) *cos(p1 latitude)*cos(p2 latitude)*haversin(delta longitude))
	haversineTheta := (haversine(p2.phi-p1.phi) + math.Cos(p1.phi)*math.Cos(p2.phi)*haversine(p2.lambda-p1.lambda))
	return 2 * EarthRadius * math.Asin(math.Sqrt(haversineTheta))
}

func GetDistance(p1 Coordinates, p2 Coordinates) float64 {
	return Distance(GetRadian(p1), GetRadian(p2))
}
