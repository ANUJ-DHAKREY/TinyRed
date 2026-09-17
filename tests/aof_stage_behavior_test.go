package tests

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// defaultMaxAOFStage is 0 because none of the AOF persistence stages are
// implemented in the server yet; all AOF tests skip by default until the
// server grows the corresponding feature (set TINYRED_AOF_STAGE to opt in).
func requireAOFStage(t *testing.T) {
	t.Helper()
	requirePhase(t, phaseAOF)
}

// waitForPath polls until os.Stat(path) succeeds (or fails after timeout),
// returning the resulting os.FileInfo. Useful because directory/file
// creation at startup can race the point where the test process observes it.
func waitForPath(t *testing.T, path string, timeout time.Duration) os.FileInfo {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		info, err := os.Stat(path)
		if err == nil {
			return info
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("path %s did not appear within %v: %v", path, timeout, lastErr)
	return nil
}

// pathShouldNotExist asserts that path does not exist after waiting briefly,
// giving any (incorrect) background creation logic a chance to run first.
func pathShouldNotExist(t *testing.T, path string, wait time.Duration) {
	t.Helper()
	time.Sleep(wait)
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected path %s to not exist, but it does", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error stat-ing %s: %v", path, err)
	}
}

// waitForFileContains polls a file until its contents contain want (or the
// timeout elapses), returning the file's contents at the point the
// condition was satisfied. Needed because AOF writes happen asynchronously
// relative to when the test process reads the file back.
func waitForFileContains(t *testing.T, path, want string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastContent []byte
	var lastErr error
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
		} else {
			lastContent = data
			if strings.Contains(string(data), want) {
				return string(data)
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("failed to read %s within %v: %v", path, timeout, lastErr)
	}
	t.Fatalf("file %s did not contain %q within %v; got %q", path, want, timeout, string(lastContent))
	return ""
}

// aofReadConfig sends CONFIG GET <key> and returns the 2-element [name, value] array.
func aofReadConfig(t *testing.T, r *bufio.Reader) []string {
	t.Helper()
	return listReadRESPArray(t, r)
}

// ========== Stage 1: Default AOF Options ==========

func TestConfigGetReturnsDefaultAOFOptions_Stage01DefaultOptions(t *testing.T) {
	requireAOFStage(t)
	// Scenario: with no AOF-related flags, CONFIG GET returns the documented
	// default values for appendonly/appenddirname/appendfilename/appendfsync.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "CONFIG", "GET", "appendonly")
	elems := aofReadConfig(t, r)
	if len(elems) != 2 || elems[0] != "appendonly" || elems[1] != "no" {
		t.Fatalf("CONFIG GET appendonly: expected [appendonly no], got %v", elems)
	}

	writeRESPArray(t, conn, "CONFIG", "GET", "appenddirname")
	elems = aofReadConfig(t, r)
	if len(elems) != 2 || elems[0] != "appenddirname" || elems[1] != "appendonlydir" {
		t.Fatalf("CONFIG GET appenddirname: expected [appenddirname appendonlydir], got %v", elems)
	}

	writeRESPArray(t, conn, "CONFIG", "GET", "appendfilename")
	elems = aofReadConfig(t, r)
	if len(elems) != 2 || elems[0] != "appendfilename" || elems[1] != "appendonly.aof" {
		t.Fatalf("CONFIG GET appendfilename: expected [appendfilename appendonly.aof], got %v", elems)
	}

	writeRESPArray(t, conn, "CONFIG", "GET", "appendfsync")
	elems = aofReadConfig(t, r)
	if len(elems) != 2 || elems[0] != "appendfsync" || elems[1] != "everysec" {
		t.Fatalf("CONFIG GET appendfsync: expected [appendfsync everysec], got %v", elems)
	}
}

// ========== Stage 2: AOF Options from Flags ==========

func TestConfigGetReflectsAOFFlags_Stage02OptionsFromFlags(t *testing.T) {
	requireAOFStage(t)
	// Scenario: --appendonly yes and --appenddirname mydir override the defaults.
	sp := startTinyRedWithArgs(t, "--appendonly", "yes", "--appenddirname", "mydir")
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "CONFIG", "GET", "appendonly")
	elems := aofReadConfig(t, r)
	if len(elems) != 2 || elems[0] != "appendonly" || elems[1] != "yes" {
		t.Fatalf("CONFIG GET appendonly: expected [appendonly yes], got %v", elems)
	}

	writeRESPArray(t, conn, "CONFIG", "GET", "appenddirname")
	elems = aofReadConfig(t, r)
	if len(elems) != 2 || elems[0] != "appenddirname" || elems[1] != "mydir" {
		t.Fatalf("CONFIG GET appenddirname: expected [appenddirname mydir], got %v", elems)
	}
}

// ========== Stage 3: Create Append-Only Directory ==========

func TestAppendOnlyDirectoryCreatedWhenEnabled_Stage03CreateAppendDir(t *testing.T) {
	requireAOFStage(t)
	// Scenario: starting with --appendonly yes creates <dir>/appendonlydir/.
	dir := t.TempDir()
	sp := startTinyRedWithArgs(t, "--dir", dir, "--appendonly", "yes")
	_ = sp

	info := waitForPath(t, filepath.Join(dir, "appendonlydir"), time.Second)
	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", filepath.Join(dir, "appendonlydir"))
	}
}

func TestAppendOnlyDirectoryNotCreatedWhenDisabled_Stage03CreateAppendDir(t *testing.T) {
	requireAOFStage(t)
	// Scenario: without --appendonly yes, no append-only directory is created.
	dir := t.TempDir()
	sp := startTinyRedWithArgs(t, "--dir", dir, "--appendonly", "no")
	_ = sp

	pathShouldNotExist(t, filepath.Join(dir, "appendonlydir"), 200*time.Millisecond)
}

// ========== Stage 4: Create Append-Only File ==========

func TestAppendOnlyFileCreatedEmpty_Stage04CreateAppendFile(t *testing.T) {
	requireAOFStage(t)
	// Scenario: starting with --appendonly yes creates an empty incr AOF file.
	dir := t.TempDir()
	sp := startTinyRedWithArgs(t, "--dir", dir, "--appendonly", "yes")
	_ = sp

	path := filepath.Join(dir, "appendonlydir", "appendonly.aof.1.incr.aof")
	info := waitForPath(t, path, time.Second)
	if info.IsDir() {
		t.Fatalf("expected %s to be a file, not a directory", path)
	}
	if info.Size() != 0 {
		t.Fatalf("expected %s to be empty, got size %d", path, info.Size())
	}
}

// ========== Stage 5: Create Manifest File ==========

func TestManifestFileCreatedWithExpectedContents_Stage05CreateManifest(t *testing.T) {
	requireAOFStage(t)
	// Scenario: starting with --appendonly yes creates a manifest file pointing
	// at the seq-1 incr AOF file.
	dir := t.TempDir()
	sp := startTinyRedWithArgs(t, "--dir", dir, "--appendonly", "yes")
	_ = sp

	path := filepath.Join(dir, "appendonlydir", "appendonly.aof.manifest")
	waitForPath(t, path, time.Second)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read manifest file: %v", err)
	}
	content := string(data)
	want := "file appendonly.aof.1.incr.aof seq 1 type i"
	if strings.TrimRight(content, "\n") != want {
		t.Fatalf("manifest content: expected %q, got %q", want, strings.TrimRight(content, "\n"))
	}
	if !strings.HasSuffix(content, "\n") {
		t.Fatalf("manifest content should end with a newline, got %q", content)
	}
}

// ========== Stage 6: Write a Single Command ==========

func TestSingleWriteCommandAppendedToAOF_Stage06WriteSingleCommand(t *testing.T) {
	requireAOFStage(t)
	// Scenario: SET foo 100 is appended to the AOF file in RESP format.
	dir := t.TempDir()
	sp := startTinyRedWithArgs(t, "--dir", dir, "--appendonly", "yes", "--appendfsync", "always")
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "100")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET foo 100: expected +OK, got %q", got)
	}

	path := filepath.Join(dir, "appendonlydir", "appendonly.aof.1.incr.aof")
	want := "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\n100\r\n"
	content := waitForFileContains(t, path, want, time.Second)
	if content != want {
		t.Fatalf("AOF content: expected %q, got %q", want, content)
	}
}

// ========== Stage 7: Write Multiple Commands ==========

func TestMultipleWriteCommandsAppendedInOrder_Stage07WriteMultipleCommands(t *testing.T) {
	requireAOFStage(t)
	// Scenario: two SET commands are appended to the AOF file, concatenated
	// in the order they were issued.
	dir := t.TempDir()
	sp := startTinyRedWithArgs(t, "--dir", dir, "--appendonly", "yes", "--appendfsync", "always")
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "a", "1")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET a 1: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "SET", "b", "2")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET b 2: expected +OK, got %q", got)
	}

	path := filepath.Join(dir, "appendonlydir", "appendonly.aof.1.incr.aof")
	want := "*3\r\n$3\r\nSET\r\n$1\r\na\r\n$1\r\n1\r\n" + "*3\r\n$3\r\nSET\r\n$1\r\nb\r\n$1\r\n2\r\n"
	content := waitForFileContains(t, path, want, time.Second)
	if content != want {
		t.Fatalf("AOF content: expected %q, got %q", want, content)
	}
}

// ========== Stage 8: Filter Write Commands ==========

func TestOnlyWriteCommandsAppendedToAOF_Stage08FilterWriteCommands(t *testing.T) {
	requireAOFStage(t)
	// Scenario: SET is logged to the AOF file, but GET/PING/ECHO (read-only or
	// non-persisting commands) are filtered out.
	dir := t.TempDir()
	sp := startTinyRedWithArgs(t, "--dir", dir, "--appendonly", "yes", "--appendfsync", "always")
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "x", "1")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET x 1: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "GET", "x")
	if got := readBulkString(t, r); got != "$1\r\n1\r\n" {
		t.Fatalf("GET x: expected $1\\r\\n1\\r\\n, got %q", got)
	}

	writeRESPArray(t, conn, "PING")
	if got := readLine(t, r); got != "+PONG\r\n" {
		t.Fatalf("PING: expected +PONG, got %q", got)
	}

	writeRESPArray(t, conn, "ECHO", "hi")
	if got := readBulkString(t, r); got != "$2\r\nhi\r\n" {
		t.Fatalf("ECHO hi: expected $2\\r\\nhi\\r\\n, got %q", got)
	}

	path := filepath.Join(dir, "appendonlydir", "appendonly.aof.1.incr.aof")
	want := "*3\r\n$3\r\nSET\r\n$1\r\nx\r\n$1\r\n1\r\n"
	content := waitForFileContains(t, path, want, time.Second)

	if content != want {
		t.Fatalf("AOF content: expected only the SET command %q, got %q", want, content)
	}
	if strings.Contains(content, "GET") {
		t.Fatalf("AOF content should not contain GET, got %q", content)
	}
	if strings.Contains(content, "PING") {
		t.Fatalf("AOF content should not contain PING, got %q", content)
	}
	if strings.Contains(content, "ECHO") {
		t.Fatalf("AOF content should not contain ECHO, got %q", content)
	}
}

// ========== Stage 9: Replay a Single Command ==========

func TestReplaySingleCommandFromAOFOnStartup_Stage09ReplaySingleCommand(t *testing.T) {
	requireAOFStage(t)
	// Scenario: a pre-existing manifest + incr AOF file encoding SET foo bar
	// is replayed into memory when the server starts.
	dir := t.TempDir()
	appendDir := filepath.Join(dir, "appendonlydir")
	if err := os.MkdirAll(appendDir, 0755); err != nil {
		t.Fatalf("failed to create fixture append dir: %v", err)
	}

	manifest := "file appendonly.aof.1.incr.aof seq 1 type i\n"
	if err := os.WriteFile(filepath.Join(appendDir, "appendonly.aof.manifest"), []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to write fixture manifest: %v", err)
	}

	incr := "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"
	if err := os.WriteFile(filepath.Join(appendDir, "appendonly.aof.1.incr.aof"), []byte(incr), 0644); err != nil {
		t.Fatalf("failed to write fixture incr AOF file: %v", err)
	}

	sp := startTinyRedWithArgs(t, "--dir", dir, "--appendonly", "yes")
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "GET", "foo")
	if got := readBulkString(t, r); got != "$3\r\nbar\r\n" {
		t.Fatalf("GET foo after AOF replay: expected $3\\r\\nbar\\r\\n, got %q", got)
	}
}

// ========== Stage 10: Replay Multiple Commands ==========

func TestReplayMultipleCommandsFromAOFOnStartup_Stage10ReplayMultipleCommands(t *testing.T) {
	requireAOFStage(t)
	// Scenario: a pre-existing incr AOF file encoding two SET commands is
	// fully replayed (RESP framing needs no separators between commands).
	dir := t.TempDir()
	appendDir := filepath.Join(dir, "appendonlydir")
	if err := os.MkdirAll(appendDir, 0755); err != nil {
		t.Fatalf("failed to create fixture append dir: %v", err)
	}

	manifest := "file appendonly.aof.1.incr.aof seq 1 type i\n"
	if err := os.WriteFile(filepath.Join(appendDir, "appendonly.aof.manifest"), []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to write fixture manifest: %v", err)
	}

	incr := "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n" + "*3\r\n$3\r\nSET\r\n$3\r\nbaz\r\n$3\r\nqux\r\n"
	if err := os.WriteFile(filepath.Join(appendDir, "appendonly.aof.1.incr.aof"), []byte(incr), 0644); err != nil {
		t.Fatalf("failed to write fixture incr AOF file: %v", err)
	}

	sp := startTinyRedWithArgs(t, "--dir", dir, "--appendonly", "yes")
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "GET", "foo")
	if got := readBulkString(t, r); got != "$3\r\nbar\r\n" {
		t.Fatalf("GET foo after AOF replay: expected $3\\r\\nbar\\r\\n, got %q", got)
	}

	writeRESPArray(t, conn, "GET", "baz")
	if got := readBulkString(t, r); got != "$3\r\nqux\r\n" {
		t.Fatalf("GET baz after AOF replay: expected $3\\r\\nqux\\r\\n, got %q", got)
	}
}
