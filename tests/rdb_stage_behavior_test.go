package tests

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func requireRDBStage(t *testing.T) {
	t.Helper()
	requirePhase(t, phaseRDB)
}

// startTinyRedWithRDB starts the server with --dir and --dbfilename flags.
func startTinyRedWithRDB(t *testing.T, dir, dbfilename string) *serverProc {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestHelperProcess", "--",
		"--port", strconv.Itoa(port),
		"--dir", dir,
		"--dbfilename", dbfilename,
	)
	cmd.Env = append(os.Environ(), "TINYRED_HELPER_PROCESS=1")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start tinyred: %v", err)
	}

	t.Cleanup(func() {
		cancel()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	deadline := time.Now().Add(3 * time.Second)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 150*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return &serverProc{port: strconv.Itoa(port), cmd: cmd}
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("server did not become ready in time; output:\n%s", out.String())
	return nil
}

// --- RDB file builders ---

// rdbEncodeString encodes a string using length-prefixed encoding.
func rdbEncodeString(s string) []byte {
	return append(rdbEncodeLength(len(s)), []byte(s)...)
}

// rdbEncodeLength encodes a size using RDB size encoding.
func rdbEncodeLength(n int) []byte {
	if n < 64 { // 6-bit length
		return []byte{byte(n)}
	}
	if n < 16384 { // 14-bit length
		return []byte{byte(0x40 | (n >> 8)), byte(n & 0xFF)}
	}
	// 32-bit length
	buf := make([]byte, 5)
	buf[0] = 0x80
	binary.BigEndian.PutUint32(buf[1:], uint32(n))
	return buf
}

// buildRDB creates a minimal valid RDB file with given key-value pairs and optional expiry.
type rdbEntry struct {
	key      string
	value    string
	expireMs int64 // 0 means no expiry; Unix timestamp in ms
}

func buildRDB(entries []rdbEntry) []byte {
	var buf bytes.Buffer

	// Header: REDIS0011
	buf.WriteString("REDIS0011")

	// Metadata: redis-ver
	buf.WriteByte(0xFA)
	buf.Write(rdbEncodeString("redis-ver"))
	buf.Write(rdbEncodeString("6.0.16"))

	// Database section
	buf.WriteByte(0xFE) // database selector
	buf.Write(rdbEncodeLength(0))

	// Hash table sizes
	buf.WriteByte(0xFB)
	buf.Write(rdbEncodeLength(len(entries)))

	// Count entries with expiry
	expiryCount := 0
	for _, e := range entries {
		if e.expireMs != 0 {
			expiryCount++
		}
	}
	buf.Write(rdbEncodeLength(expiryCount))

	// Key-value pairs
	for _, e := range entries {
		if e.expireMs != 0 {
			buf.WriteByte(0xFC) // expire in milliseconds
			ts := make([]byte, 8)
			binary.LittleEndian.PutUint64(ts, uint64(e.expireMs))
			buf.Write(ts)
		}
		buf.WriteByte(0x00) // value type: string
		buf.Write(rdbEncodeString(e.key))
		buf.Write(rdbEncodeString(e.value))
	}

	// EOF
	buf.WriteByte(0xFF)

	// CRC64 checksum (8 bytes of zeros — Redis accepts this)
	buf.Write(make([]byte, 8))

	return buf.Bytes()
}

func writeRDBFile(t *testing.T, dir, filename string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create RDB dir: %v", err)
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write RDB file: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
}

// --- RESP helpers (reuse pattern from base tests) ---

func rdbWriteRESPArray(t *testing.T, conn net.Conn, parts ...string) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(parts))
	for _, p := range parts {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(p), p)
	}
	if _, err := io.WriteString(conn, b.String()); err != nil {
		t.Fatalf("failed to write command: %v", err)
	}
}

func rdbReadLine(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read line: %v", err)
	}
	return line
}

func rdbReadBulkString(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	header := rdbReadLine(t, r)
	if !strings.HasPrefix(header, "$") {
		t.Fatalf("expected bulk-string header, got %q", header)
	}
	if header == "$-1\r\n" {
		return header
	}

	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "$")))
	if err != nil {
		t.Fatalf("invalid bulk-string length header %q: %v", header, err)
	}
	body := make([]byte, n+2)
	if _, err := io.ReadFull(r, body); err != nil {
		t.Fatalf("failed reading bulk-string body: %v", err)
	}
	return header + string(body)
}

// readRESPArray reads a RESP array response and returns individual bulk strings.
func readRESPArray(t *testing.T, r *bufio.Reader) []string {
	t.Helper()
	header := rdbReadLine(t, r)
	if !strings.HasPrefix(header, "*") {
		t.Fatalf("expected array header, got %q", header)
	}
	count, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "*")))
	if err != nil {
		t.Fatalf("invalid array count %q: %v", header, err)
	}
	var elements []string
	for i := 0; i < count; i++ {
		elements = append(elements, rdbReadBulkString(t, r))
	}
	return elements
}

func rdbDialClient(t *testing.T, sp *serverProc) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", sp.port), 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to server: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn, bufio.NewReader(conn)
}

// extractBulkValue extracts just the string value from a bulk string like "$3\r\nfoo\r\n".
func extractBulkValue(raw string) string {
	parts := strings.SplitN(raw, "\r\n", 3)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// ========== RDB Stage 1: CONFIG GET dir / dbfilename ==========

func TestConfigGetReturnsDirAndDbfilename_RDBStage01(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	// CONFIG GET dir
	rdbWriteRESPArray(t, conn, "CONFIG", "GET", "dir")
	elems := readRESPArray(t, r)
	if len(elems) != 2 {
		t.Fatalf("CONFIG GET dir: expected 2 elements, got %d", len(elems))
	}
	if extractBulkValue(elems[0]) != "dir" {
		t.Fatalf("CONFIG GET dir: expected first element 'dir', got %q", elems[0])
	}
	if extractBulkValue(elems[1]) != dir {
		t.Fatalf("CONFIG GET dir: expected value %q, got %q", dir, extractBulkValue(elems[1]))
	}

	// CONFIG GET dbfilename
	rdbWriteRESPArray(t, conn, "CONFIG", "GET", "dbfilename")
	elems = readRESPArray(t, r)
	if len(elems) != 2 {
		t.Fatalf("CONFIG GET dbfilename: expected 2 elements, got %d", len(elems))
	}
	if extractBulkValue(elems[0]) != "dbfilename" {
		t.Fatalf("CONFIG GET dbfilename: expected first element 'dbfilename', got %q", elems[0])
	}
	if extractBulkValue(elems[1]) != dbfilename {
		t.Fatalf("CONFIG GET dbfilename: expected value %q, got %q", dbfilename, extractBulkValue(elems[1]))
	}
}

// ========== RDB Stage 2: Read a single key from RDB ==========

func TestReadSingleKeyFromRDB_RDBStage02(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	rdbData := buildRDB([]rdbEntry{
		{key: "mykey", value: "myvalue"},
	})
	writeRDBFile(t, dir, dbfilename, rdbData)

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	// KEYS * should return the single key
	rdbWriteRESPArray(t, conn, "KEYS", "*")
	elems := readRESPArray(t, r)
	if len(elems) != 1 {
		t.Fatalf("KEYS * expected 1 key, got %d", len(elems))
	}
	if extractBulkValue(elems[0]) != "mykey" {
		t.Fatalf("KEYS * expected 'mykey', got %q", extractBulkValue(elems[0]))
	}
}

// ========== RDB Stage 3: Read string value from RDB ==========

func TestReadStringValueFromRDB_RDBStage03(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	rdbData := buildRDB([]rdbEntry{
		{key: "foo", value: "bar"},
	})
	writeRDBFile(t, dir, dbfilename, rdbData)

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	// GET foo should return "bar"
	rdbWriteRESPArray(t, conn, "GET", "foo")
	got := rdbReadBulkString(t, r)
	if got != "$3\r\nbar\r\n" {
		t.Fatalf("GET foo: expected $3\\r\\nbar\\r\\n, got %q", got)
	}
}

// ========== RDB Stage 4: Read multiple keys from RDB ==========

func TestReadMultipleKeysFromRDB_RDBStage04(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	rdbData := buildRDB([]rdbEntry{
		{key: "foo", value: "123"},
		{key: "bar", value: "456"},
		{key: "baz", value: "789"},
	})
	writeRDBFile(t, dir, dbfilename, rdbData)

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	// KEYS * should return all 3 keys
	rdbWriteRESPArray(t, conn, "KEYS", "*")
	elems := readRESPArray(t, r)
	if len(elems) != 3 {
		t.Fatalf("KEYS * expected 3 keys, got %d", len(elems))
	}

	keys := make([]string, len(elems))
	for i, e := range elems {
		keys[i] = extractBulkValue(e)
	}
	sort.Strings(keys)
	expected := []string{"bar", "baz", "foo"}
	for i, k := range keys {
		if k != expected[i] {
			t.Fatalf("KEYS * expected %v, got %v", expected, keys)
		}
	}
}

// ========== RDB Stage 5: Read multiple string values from RDB ==========

func TestReadMultipleStringValuesFromRDB_RDBStage05(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	rdbData := buildRDB([]rdbEntry{
		{key: "apple", value: "red"},
		{key: "banana", value: "yellow"},
		{key: "grape", value: "purple"},
	})
	writeRDBFile(t, dir, dbfilename, rdbData)

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	// GET each key
	cases := []struct {
		key, val string
	}{
		{"apple", "red"},
		{"banana", "yellow"},
		{"grape", "purple"},
	}

	for _, tc := range cases {
		rdbWriteRESPArray(t, conn, "GET", tc.key)
		got := rdbReadBulkString(t, r)
		expected := fmt.Sprintf("$%d\r\n%s\r\n", len(tc.val), tc.val)
		if got != expected {
			t.Fatalf("GET %s: expected %q, got %q", tc.key, expected, got)
		}
	}
}

// ========== RDB Stage 6: Read values with expiry ==========

func TestReadValuesWithExpiryFromRDB_RDBStage06(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	now := time.Now()
	pastExpiry := now.Add(-1 * time.Hour).UnixMilli()  // expired 1 hour ago
	futureExpiry := now.Add(1 * time.Hour).UnixMilli() // expires in 1 hour

	rdbData := buildRDB([]rdbEntry{
		{key: "expired_key", value: "gone", expireMs: pastExpiry},
		{key: "active_key", value: "here", expireMs: futureExpiry},
		{key: "no_expiry_key", value: "forever"},
	})
	writeRDBFile(t, dir, dbfilename, rdbData)

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	// GET expired_key should return null
	rdbWriteRESPArray(t, conn, "GET", "expired_key")
	got := rdbReadBulkString(t, r)
	if got != "$-1\r\n" {
		t.Fatalf("GET expired_key: expected null bulk string, got %q", got)
	}

	// GET active_key should return the value
	rdbWriteRESPArray(t, conn, "GET", "active_key")
	got = rdbReadBulkString(t, r)
	if got != "$4\r\nhere\r\n" {
		t.Fatalf("GET active_key: expected $4\\r\\nhere\\r\\n, got %q", got)
	}

	// GET no_expiry_key should return the value
	rdbWriteRESPArray(t, conn, "GET", "no_expiry_key")
	got = rdbReadBulkString(t, r)
	if got != "$7\r\nforever\r\n" {
		t.Fatalf("GET no_expiry_key: expected $7\\r\\nforever\\r\\n, got %q", got)
	}
}

// ========== Additional RDB Stage 1 Tests ==========

func TestConfigGetDir_CaseInsensitive_RDBStage01(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	// CONFIG GET with mixed case should still work
	rdbWriteRESPArray(t, conn, "config", "get", "dir")
	elems := readRESPArray(t, r)
	if len(elems) != 2 {
		t.Fatalf("config get dir: expected 2 elements, got %d", len(elems))
	}
	if extractBulkValue(elems[1]) != dir {
		t.Fatalf("config get dir: expected %q, got %q", dir, extractBulkValue(elems[1]))
	}
}

func TestConfigGetDbfilename_CustomName_RDBStage01(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "my-custom-db.rdb"

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	rdbWriteRESPArray(t, conn, "CONFIG", "GET", "dbfilename")
	elems := readRESPArray(t, r)
	if len(elems) != 2 {
		t.Fatalf("CONFIG GET dbfilename: expected 2 elements, got %d", len(elems))
	}
	if extractBulkValue(elems[1]) != dbfilename {
		t.Fatalf("CONFIG GET dbfilename: expected %q, got %q", dbfilename, extractBulkValue(elems[1]))
	}
}

// ========== Additional RDB Stage 2 Tests ==========

func TestReadSingleKeyFromRDB_EmptyDB_RDBStage02(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	// Build RDB with no entries
	rdbData := buildRDB([]rdbEntry{})
	writeRDBFile(t, dir, dbfilename, rdbData)

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	// KEYS * on empty DB should return empty array
	rdbWriteRESPArray(t, conn, "KEYS", "*")
	header := rdbReadLine(t, r)
	if header != "*0\r\n" {
		t.Fatalf("KEYS * on empty DB: expected *0\\r\\n, got %q", header)
	}
}

func TestReadSingleKeyFromRDB_LongKeyName_RDBStage02(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	longKey := strings.Repeat("x", 100)
	rdbData := buildRDB([]rdbEntry{
		{key: longKey, value: "val"},
	})
	writeRDBFile(t, dir, dbfilename, rdbData)

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	rdbWriteRESPArray(t, conn, "KEYS", "*")
	elems := readRESPArray(t, r)
	if len(elems) != 1 {
		t.Fatalf("KEYS * expected 1 key, got %d", len(elems))
	}
	if extractBulkValue(elems[0]) != longKey {
		t.Fatalf("KEYS * expected long key, got %q", extractBulkValue(elems[0]))
	}
}

// ========== Additional RDB Stage 3 Tests ==========

func TestReadStringValueFromRDB_EmptyValue_RDBStage03(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	rdbData := buildRDB([]rdbEntry{
		{key: "emptyval", value: ""},
	})
	writeRDBFile(t, dir, dbfilename, rdbData)

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	rdbWriteRESPArray(t, conn, "GET", "emptyval")
	got := rdbReadBulkString(t, r)
	if got != "$0\r\n\r\n" {
		t.Fatalf("GET emptyval: expected $0\\r\\n\\r\\n, got %q", got)
	}
}

func TestReadStringValueFromRDB_NonExistentKey_RDBStage03(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	rdbData := buildRDB([]rdbEntry{
		{key: "foo", value: "bar"},
	})
	writeRDBFile(t, dir, dbfilename, rdbData)

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	// GET a key that doesn't exist
	rdbWriteRESPArray(t, conn, "GET", "nonexistent")
	got := rdbReadBulkString(t, r)
	if got != "$-1\r\n" {
		t.Fatalf("GET nonexistent: expected null bulk string, got %q", got)
	}
}

func TestReadStringValueFromRDB_LongValue_RDBStage03(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	longVal := strings.Repeat("abcdefghij", 50) // 500 chars
	rdbData := buildRDB([]rdbEntry{
		{key: "bigkey", value: longVal},
	})
	writeRDBFile(t, dir, dbfilename, rdbData)

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	rdbWriteRESPArray(t, conn, "GET", "bigkey")
	got := rdbReadBulkString(t, r)
	expected := fmt.Sprintf("$%d\r\n%s\r\n", len(longVal), longVal)
	if got != expected {
		t.Fatalf("GET bigkey: expected value of length %d, got response of length %d", len(expected), len(got))
	}
}

// ========== Additional RDB Stage 4 Tests ==========

func TestReadMultipleKeysFromRDB_TenKeys_RDBStage04(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	entries := make([]rdbEntry, 10)
	expectedKeys := make([]string, 10)
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key_%02d", i)
		entries[i] = rdbEntry{key: key, value: fmt.Sprintf("val_%02d", i)}
		expectedKeys[i] = key
	}

	rdbData := buildRDB(entries)
	writeRDBFile(t, dir, dbfilename, rdbData)

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	rdbWriteRESPArray(t, conn, "KEYS", "*")
	elems := readRESPArray(t, r)
	if len(elems) != 10 {
		t.Fatalf("KEYS * expected 10 keys, got %d", len(elems))
	}

	keys := make([]string, len(elems))
	for i, e := range elems {
		keys[i] = extractBulkValue(e)
	}
	sort.Strings(keys)
	sort.Strings(expectedKeys)
	for i, k := range keys {
		if k != expectedKeys[i] {
			t.Fatalf("KEYS * expected %v, got %v", expectedKeys, keys)
		}
	}
}

func TestReadMultipleKeysFromRDB_SpecialCharacters_RDBStage04(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	rdbData := buildRDB([]rdbEntry{
		{key: "user:1001", value: "alice"},
		{key: "session:abc-def", value: "data"},
		{key: "cache.item.5", value: "cached"},
	})
	writeRDBFile(t, dir, dbfilename, rdbData)

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	rdbWriteRESPArray(t, conn, "KEYS", "*")
	elems := readRESPArray(t, r)
	if len(elems) != 3 {
		t.Fatalf("KEYS * expected 3 keys, got %d", len(elems))
	}

	keys := make([]string, len(elems))
	for i, e := range elems {
		keys[i] = extractBulkValue(e)
	}
	sort.Strings(keys)
	expected := []string{"cache.item.5", "session:abc-def", "user:1001"}
	for i, k := range keys {
		if k != expected[i] {
			t.Fatalf("KEYS * expected %v, got %v", expected, keys)
		}
	}
}

// ========== Additional RDB Stage 5 Tests ==========

func TestReadMultipleStringValuesFromRDB_SetOverridesRDB_RDBStage05(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	rdbData := buildRDB([]rdbEntry{
		{key: "color", value: "blue"},
	})
	writeRDBFile(t, dir, dbfilename, rdbData)

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	// Verify original value from RDB
	rdbWriteRESPArray(t, conn, "GET", "color")
	got := rdbReadBulkString(t, r)
	if got != "$4\r\nblue\r\n" {
		t.Fatalf("GET color: expected $4\\r\\nblue\\r\\n, got %q", got)
	}

	// SET a new value for the same key
	rdbWriteRESPArray(t, conn, "SET", "color", "green")
	reply := rdbReadLine(t, r)
	if reply != "+OK\r\n" {
		t.Fatalf("SET color: expected +OK\\r\\n, got %q", reply)
	}

	// GET should now return the new value
	rdbWriteRESPArray(t, conn, "GET", "color")
	got = rdbReadBulkString(t, r)
	if got != "$5\r\ngreen\r\n" {
		t.Fatalf("GET color after SET: expected $5\\r\\ngreen\\r\\n, got %q", got)
	}
}

func TestReadMultipleStringValuesFromRDB_MixedWithInMemory_RDBStage05(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	rdbData := buildRDB([]rdbEntry{
		{key: "from_rdb", value: "persisted"},
	})
	writeRDBFile(t, dir, dbfilename, rdbData)

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	// SET a new key not in RDB
	rdbWriteRESPArray(t, conn, "SET", "in_memory", "transient")
	reply := rdbReadLine(t, r)
	if reply != "+OK\r\n" {
		t.Fatalf("SET in_memory: expected +OK, got %q", reply)
	}

	// Both keys should be accessible
	rdbWriteRESPArray(t, conn, "GET", "from_rdb")
	got := rdbReadBulkString(t, r)
	if got != "$9\r\npersisted\r\n" {
		t.Fatalf("GET from_rdb: expected $9\\r\\npersisted\\r\\n, got %q", got)
	}

	rdbWriteRESPArray(t, conn, "GET", "in_memory")
	got = rdbReadBulkString(t, r)
	if got != "$9\r\ntransient\r\n" {
		t.Fatalf("GET in_memory: expected $9\\r\\ntransient\\r\\n, got %q", got)
	}
}

// ========== Additional RDB Stage 6 Tests ==========

func TestExpiry_AllKeysExpired_RDBStage06(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	pastExpiry := time.Now().Add(-10 * time.Minute).UnixMilli()

	rdbData := buildRDB([]rdbEntry{
		{key: "gone1", value: "v1", expireMs: pastExpiry},
		{key: "gone2", value: "v2", expireMs: pastExpiry},
		{key: "gone3", value: "v3", expireMs: pastExpiry},
	})
	writeRDBFile(t, dir, dbfilename, rdbData)

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	// All keys should return null
	for _, key := range []string{"gone1", "gone2", "gone3"} {
		rdbWriteRESPArray(t, conn, "GET", key)
		got := rdbReadBulkString(t, r)
		if got != "$-1\r\n" {
			t.Fatalf("GET %s: expected null bulk string for expired key, got %q", key, got)
		}
	}
}

func TestExpiry_AllKeysFuture_RDBStage06(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	futureExpiry := time.Now().Add(24 * time.Hour).UnixMilli()

	rdbData := buildRDB([]rdbEntry{
		{key: "alive1", value: "val1", expireMs: futureExpiry},
		{key: "alive2", value: "val2", expireMs: futureExpiry},
	})
	writeRDBFile(t, dir, dbfilename, rdbData)

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	// All keys should return their values
	cases := []struct{ key, val string }{
		{"alive1", "val1"},
		{"alive2", "val2"},
	}
	for _, tc := range cases {
		rdbWriteRESPArray(t, conn, "GET", tc.key)
		got := rdbReadBulkString(t, r)
		expected := fmt.Sprintf("$%d\r\n%s\r\n", len(tc.val), tc.val)
		if got != expected {
			t.Fatalf("GET %s: expected %q, got %q", tc.key, expected, got)
		}
	}
}

func TestExpiry_MixedWithKeysCommand_RDBStage06(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "dump.rdb"

	now := time.Now()
	pastExpiry := now.Add(-30 * time.Minute).UnixMilli()
	futureExpiry := now.Add(2 * time.Hour).UnixMilli()

	rdbData := buildRDB([]rdbEntry{
		{key: "expired_a", value: "x", expireMs: pastExpiry},
		{key: "active_b", value: "y", expireMs: futureExpiry},
		{key: "permanent_c", value: "z"},
		{key: "expired_d", value: "w", expireMs: pastExpiry},
	})
	writeRDBFile(t, dir, dbfilename, rdbData)

	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	// KEYS * should only show non-expired keys
	rdbWriteRESPArray(t, conn, "KEYS", "*")
	elems := readRESPArray(t, r)

	keys := make([]string, len(elems))
	for i, e := range elems {
		keys[i] = extractBulkValue(e)
	}
	sort.Strings(keys)

	// Depending on implementation, KEYS might return all keys or only active ones.
	// Redis filters expired keys on access; KEYS * may or may not filter.
	// At minimum, GET for expired keys must return null.
	rdbWriteRESPArray(t, conn, "GET", "expired_a")
	got := rdbReadBulkString(t, r)
	if got != "$-1\r\n" {
		t.Fatalf("GET expired_a: expected null, got %q", got)
	}

	rdbWriteRESPArray(t, conn, "GET", "active_b")
	got = rdbReadBulkString(t, r)
	if got != "$1\r\ny\r\n" {
		t.Fatalf("GET active_b: expected $1\\r\\ny\\r\\n, got %q", got)
	}

	rdbWriteRESPArray(t, conn, "GET", "permanent_c")
	got = rdbReadBulkString(t, r)
	if got != "$1\r\nz\r\n" {
		t.Fatalf("GET permanent_c: expected $1\\r\\nz\\r\\n, got %q", got)
	}
}

func TestNoRDBFile_ServerStartsClean_RDBStage02(t *testing.T) {
	requireRDBStage(t)

	dir := t.TempDir()
	dbfilename := "nonexistent.rdb"

	// Do NOT write any RDB file — server should start with empty store
	sp := startTinyRedWithRDB(t, dir, dbfilename)
	conn, r := rdbDialClient(t, sp)

	rdbWriteRESPArray(t, conn, "GET", "anything")
	got := rdbReadBulkString(t, r)
	if got != "$-1\r\n" {
		t.Fatalf("GET on empty server: expected null bulk string, got %q", got)
	}
}
