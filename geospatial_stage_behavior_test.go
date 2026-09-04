package main

import (
	"bufio"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// defaultMaxGeospatialStage defaults to 0 so that, absent explicit opt-in via
// TINYRED_GEOSPATIAL_STAGE, every geospatial test in this file is skipped.
// None of GEOADD/GEOPOS/GEODIST/GEOSEARCH (nor the sorted set commands this
// phase is built on top of) are implemented yet.
const defaultMaxGeospatialStage = 0

func maxGeospatialStage() int {
	raw := strings.TrimSpace(os.Getenv("TINYRED_GEOSPATIAL_STAGE"))
	if raw == "" {
		return defaultMaxGeospatialStage
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return defaultMaxGeospatialStage
	}
	if v > 8 {
		return 8
	}
	return v
}

func requireGeospatialStage(t *testing.T, stage int) {
	t.Helper()
	if stage > maxGeospatialStage() {
		t.Skipf("skipping geospatial stage %d test; set TINYRED_GEOSPATIAL_STAGE=%d (or higher) to run", stage, stage)
	}
}

// geoReadSimpleError reads a RESP simple error line (e.g. "-ERR message\r\n")
// and returns its message with the leading '-' and trailing CRLF stripped.
func geoReadSimpleError(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	line := readLine(t, r)
	if !strings.HasPrefix(line, "-") {
		t.Fatalf("expected RESP simple error, got %q", line)
	}
	if !strings.HasSuffix(line, "\r\n") {
		t.Fatalf("expected RESP simple error to end with CRLF, got %q", line)
	}
	return strings.TrimSuffix(strings.TrimPrefix(line, "-"), "\r\n")
}

// geoReadPosArray reads the outer "*count\r\n" array header for a GEOPOS
// response and then, for each of the count elements, reads either a
// null array (*-1\r\n, represented as a nil entry) or a 2-element array of
// bulk strings (represented as a non-nil [2]string{lon, lat}).
func geoReadPosArray(t *testing.T, r *bufio.Reader, count int) []*[2]string {
	t.Helper()

	header := readLine(t, r)
	if !strings.HasPrefix(header, "*") {
		t.Fatalf("expected RESP array header, got %q", header)
	}
	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "*")))
	if err != nil {
		t.Fatalf("invalid RESP array length %q: %v", header, err)
	}
	if n != count {
		t.Fatalf("expected outer GEOPOS array of %d elements, got %d", count, n)
	}

	result := make([]*[2]string, count)
	for i := 0; i < count; i++ {
		elemHeader := readLine(t, r)
		if elemHeader == "*-1\r\n" {
			result[i] = nil
			continue
		}
		if !strings.HasPrefix(elemHeader, "*") {
			t.Fatalf("expected element array header, got %q", elemHeader)
		}
		m, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(elemHeader, "*")))
		if err != nil {
			t.Fatalf("invalid element array length %q: %v", elemHeader, err)
		}
		if m != 2 {
			t.Fatalf("expected 2-element position array, got %d elements", m)
		}
		lon := readRESPBulkStringValue(t, r)
		lat := readRESPBulkStringValue(t, r)
		result[i] = &[2]string{lon, lat}
	}
	return result
}

// geoAssertSameSet compares got and want as sets of strings, ignoring order.
func geoAssertSameSet(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d elements, got %d: got=%v want=%v", len(want), len(got), got, want)
	}
	gotSorted := append([]string(nil), got...)
	wantSorted := append([]string(nil), want...)
	sort.Strings(gotSorted)
	sort.Strings(wantSorted)
	if !sliceEqual(gotSorted, wantSorted) {
		t.Fatalf("expected set %v, got %v", want, got)
	}
}

// --- Stage 01 (doc stage 100): Respond to GEOADD ---

func TestGeoAddRespondsWithIntegerCount_Stage01GeoAddBasic(t *testing.T) {
	requireGeospatialStage(t, 1)
	// Scenario: GEOADD with a single valid location returns the count of locations added as a RESP integer.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "GEOADD", "places", "11.5030378", "48.164271", "Munich")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("GEOADD: expected 1, got %d", got)
	}
}

// --- Stage 02 (doc stage 101): Validate coordinates ---

func TestGeoAddRejectsInvalidLatitude_Stage02ValidateCoordinates(t *testing.T) {
	requireGeospatialStage(t, 2)
	// Scenario: latitude 100 is out of the valid [-85.05112878, 85.05112878] range, so GEOADD
	// must return a RESP simple error starting with "ERR" and mentioning "latitude".
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "GEOADD", "location_key", "200", "100", "foo")
	msg := geoReadSimpleError(t, r)
	if !strings.HasPrefix(msg, "ERR") {
		t.Fatalf("expected error message to start with ERR, got %q", msg)
	}
	if !strings.Contains(strings.ToLower(msg), "latitude") {
		t.Fatalf("expected error message to mention latitude, got %q", msg)
	}
}

func TestGeoAddRejectsInvalidLongitude_Stage02ValidateCoordinates(t *testing.T) {
	requireGeospatialStage(t, 2)
	// Scenario: longitude 181 is out of the valid [-180, 180] range (latitude 0.3 is valid),
	// so GEOADD must return a RESP simple error starting with "ERR" and mentioning "longitude".
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "GEOADD", "location_key", "181", "0.3", "test2")
	msg := geoReadSimpleError(t, r)
	if !strings.HasPrefix(msg, "ERR") {
		t.Fatalf("expected error message to start with ERR, got %q", msg)
	}
	if !strings.Contains(strings.ToLower(msg), "longitude") {
		t.Fatalf("expected error message to mention longitude, got %q", msg)
	}
}

func TestGeoAddAcceptsBoundaryCoordinates_Stage02ValidateCoordinates(t *testing.T) {
	requireGeospatialStage(t, 2)
	// Scenario: the boundary values -180/+180 (longitude) and -85.05112878/+85.05112878 (latitude)
	// are inclusive-valid, so GEOADD must succeed for both.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "GEOADD", "location_key", "-180", "-85.05112878", "edge1")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("GEOADD edge1 (min bounds): expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "GEOADD", "location_key", "180", "85.05112878", "edge2")
	got = readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("GEOADD edge2 (max bounds): expected 1, got %d", got)
	}
}

// --- Stage 03 (doc stage 102): Store a location ---

func TestGeoAddStoresLocationInSortedSet_Stage03StoreLocation(t *testing.T) {
	requireGeospatialStage(t, 3)
	// Scenario: GEOADD must store the member in a sorted set retrievable via ZRANGE.
	// This stage allows the score to be hardcoded to 0, so only membership is checked.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "GEOADD", "places", "2.2944692", "48.8584625", "Paris")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("GEOADD: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "ZRANGE", "places", "0", "-1")
	elems := listReadRESPArray(t, r)
	expected := []string{"Paris"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("ZRANGE places 0 -1: expected %v, got %v", expected, elems)
	}
}

// --- Stage 04 (doc stage 103): Calculate location score ---

func TestGeoAddCalculatesLocationScore_Stage04CalculateScore(t *testing.T) {
	requireGeospatialStage(t, 4)
	// Scenario: GEOADD converts latitude/longitude into a geocoded score, retrievable via ZSCORE.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "GEOADD", "places", "2.2944692", "48.8584625", "Paris")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("GEOADD Paris: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "GEOADD", "places", "-0.127758", "51.507351", "London")
	got = readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("GEOADD London: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "ZSCORE", "places", "Paris")
	raw := readRESPBulkStringValue(t, r)
	score, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		t.Fatalf("ZSCORE places Paris: could not parse %q as float: %v", raw, err)
	}
	const want = 3663832614298053
	if math.Abs(score-want) >= 1.0 {
		t.Fatalf("ZSCORE places Paris: expected approximately %v, got %v", want, score)
	}
}

// --- Stage 05 (doc stage 104): Respond to GEOPOS ---

func TestGeoPosRespondsWithStructure_Stage05GeoPosStructure(t *testing.T) {
	requireGeospatialStage(t, 5)
	// Scenario: GEOPOS on existing locations returns one entry per requested location; this
	// stage allows lon/lat to be hardcoded, so only the structural shape is asserted.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "GEOADD", "location_key", "-0.0884948", "51.506479", "London")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("GEOADD London: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "GEOADD", "location_key", "11.5030378", "48.164271", "Munich")
	got = readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("GEOADD Munich: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "GEOPOS", "location_key", "London", "Munich")
	positions := geoReadPosArray(t, r, 2)
	for i, pos := range positions {
		if pos == nil {
			t.Fatalf("GEOPOS entry %d: expected a position array, got null", i)
		}
		if _, err := strconv.ParseFloat(pos[0], 64); err != nil {
			t.Fatalf("GEOPOS entry %d: longitude %q is not a valid float: %v", i, pos[0], err)
		}
		if _, err := strconv.ParseFloat(pos[1], 64); err != nil {
			t.Fatalf("GEOPOS entry %d: latitude %q is not a valid float: %v", i, pos[1], err)
		}
	}
}

func TestGeoPosMissingLocationReturnsNullArray_Stage05GeoPosStructure(t *testing.T) {
	requireGeospatialStage(t, 5)
	// Scenario: GEOPOS for a location that doesn't exist under an existing key returns a null array entry.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "GEOADD", "location_key", "-0.0884948", "51.506479", "London")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "GEOPOS", "location_key", "missing_location")
	positions := geoReadPosArray(t, r, 1)
	if positions[0] != nil {
		t.Fatalf("GEOPOS missing_location: expected null entry, got %v", *positions[0])
	}
}

func TestGeoPosMissingKeyReturnsNullArrays_Stage05GeoPosStructure(t *testing.T) {
	requireGeospatialStage(t, 5)
	// Scenario: GEOPOS against a key that doesn't exist at all returns a null array for every requested member.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "GEOPOS", "missing_key", "London", "Munich")
	positions := geoReadPosArray(t, r, 2)
	for i, pos := range positions {
		if pos != nil {
			t.Fatalf("GEOPOS entry %d: expected null entry, got %v", i, *pos)
		}
	}
}

// --- Stage 06 (doc stage 105): Decode coordinates ---

func TestGeoPosDecodesStoredCoordinates_Stage06DecodeCoordinates(t *testing.T) {
	requireGeospatialStage(t, 6)
	// Scenario: GEOPOS must decode a precomputed geocoded score (set directly via ZADD) back
	// into longitude/latitude values, accurate to within 1e-5 of the documented expected values.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "ZADD", "location_key", "3663832614298053", "Foo")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("ZADD location_key Foo: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "GEOPOS", "location_key", "Foo")
	positions := geoReadPosArray(t, r, 1)
	if positions[0] == nil {
		t.Fatalf("GEOPOS Foo: expected a position array, got null")
	}

	lon, err := strconv.ParseFloat(positions[0][0], 64)
	if err != nil {
		t.Fatalf("GEOPOS Foo: longitude %q is not a valid float: %v", positions[0][0], err)
	}
	lat, err := strconv.ParseFloat(positions[0][1], 64)
	if err != nil {
		t.Fatalf("GEOPOS Foo: latitude %q is not a valid float: %v", positions[0][1], err)
	}

	const wantLon = 2.294471561908722
	const wantLat = 48.85846255040141
	if math.Abs(lon-wantLon) >= 1e-5 {
		t.Fatalf("GEOPOS Foo longitude: expected approximately %v, got %v", wantLon, lon)
	}
	if math.Abs(lat-wantLat) >= 1e-5 {
		t.Fatalf("GEOPOS Foo latitude: expected approximately %v, got %v", wantLat, lat)
	}
}

// --- Stage 07 (doc stage 106): Calculate distance ---

func TestGeoDistCalculatesDistanceBetweenLocations_Stage07GeoDist(t *testing.T) {
	requireGeospatialStage(t, 7)
	// Scenario: GEODIST returns the Haversine distance in meters between two members of a key,
	// as a RESP bulk string, within 1 meter of the documented expected value.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "GEOADD", "places", "11.5030378", "48.164271", "Munich")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("GEOADD Munich: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "GEOADD", "places", "2.2944692", "48.8584625", "Paris")
	got = readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("GEOADD Paris: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "GEODIST", "places", "Munich", "Paris")
	raw := readRESPBulkStringValue(t, r)
	dist, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		t.Fatalf("GEODIST places Munich Paris: could not parse %q as float: %v", raw, err)
	}
	const want = 682477.7582
	if math.Abs(dist-want) >= 1.0 {
		t.Fatalf("GEODIST places Munich Paris: expected approximately %v, got %v", want, dist)
	}
}

// --- Stage 08 (doc stage 107): Search within radius ---

func TestGeoSearchFindsSingleLocationWithinRadius_Stage08GeoSearch(t *testing.T) {
	requireGeospatialStage(t, 8)
	// Scenario: GEOSEARCH FROMLONLAT ... BYRADIUS 100000 m from (2, 48) should find only Paris.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "GEOADD", "places", "11.5030378", "48.164271", "Munich")
	_ = readRESPInteger(t, r)
	writeRESPArray(t, conn, "GEOADD", "places", "2.2944692", "48.8584625", "Paris")
	_ = readRESPInteger(t, r)
	writeRESPArray(t, conn, "GEOADD", "places", "-0.0884948", "51.506479", "London")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "GEOSEARCH", "places", "FROMLONLAT", "2", "48", "BYRADIUS", "100000", "m")
	elems := listReadRESPArray(t, r)
	expected := []string{"Paris"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("GEOSEARCH 100000m from (2,48): expected %v, got %v", expected, elems)
	}
}

func TestGeoSearchFindsMultipleLocationsAsSet_Stage08GeoSearch(t *testing.T) {
	requireGeospatialStage(t, 8)
	// Scenario: GEOSEARCH FROMLONLAT ... BYRADIUS 500000 m from (2, 48) should find Paris and
	// London, in any order.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "GEOADD", "places", "11.5030378", "48.164271", "Munich")
	_ = readRESPInteger(t, r)
	writeRESPArray(t, conn, "GEOADD", "places", "2.2944692", "48.8584625", "Paris")
	_ = readRESPInteger(t, r)
	writeRESPArray(t, conn, "GEOADD", "places", "-0.0884948", "51.506479", "London")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "GEOSEARCH", "places", "FROMLONLAT", "2", "48", "BYRADIUS", "500000", "m")
	elems := listReadRESPArray(t, r)
	geoAssertSameSet(t, elems, []string{"Paris", "London"})
}

func TestGeoSearchFindsDifferentLocationWithinRadius_Stage08GeoSearch(t *testing.T) {
	requireGeospatialStage(t, 8)
	// Scenario: GEOSEARCH FROMLONLAT ... BYRADIUS 300000 m from (11, 50) should find only Munich.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "GEOADD", "places", "11.5030378", "48.164271", "Munich")
	_ = readRESPInteger(t, r)
	writeRESPArray(t, conn, "GEOADD", "places", "2.2944692", "48.8584625", "Paris")
	_ = readRESPInteger(t, r)
	writeRESPArray(t, conn, "GEOADD", "places", "-0.0884948", "51.506479", "London")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "GEOSEARCH", "places", "FROMLONLAT", "11", "50", "BYRADIUS", "300000", "m")
	elems := listReadRESPArray(t, r)
	expected := []string{"Munich"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("GEOSEARCH 300000m from (11,50): expected %v, got %v", expected, elems)
	}
}
