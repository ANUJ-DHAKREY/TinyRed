package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"testing"
)

const defaultMaxACLAuthStage = 0

func maxACLAuthStage() int {
	raw := strings.TrimSpace(os.Getenv("TINYRED_ACL_AUTH_STAGE"))
	if raw == "" {
		return defaultMaxACLAuthStage
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return defaultMaxACLAuthStage
	}
	if v > 8 {
		return 8
	}
	return v
}

func requireACLAuthStage(t *testing.T, stage int) {
	t.Helper()
	if stage > maxACLAuthStage() {
		t.Skipf("skipping acl/auth stage %d test; set TINYRED_ACL_AUTH_STAGE=%d (or higher) to run", stage, stage)
	}
}

// aclReadGetUserResponse reads the response to ACL GETUSER: an outer RESP array
// of alternating property-name / property-value pairs, where each value is
// itself a RESP array of bulk strings (possibly empty). It returns a map from
// property name to the value slice so tests can check specific properties
// regardless of the order (or subset) the server emits them in.
func aclReadGetUserResponse(t *testing.T, r *bufio.Reader) map[string][]string {
	t.Helper()
	header := readLine(t, r)
	if !strings.HasPrefix(header, "*") {
		t.Fatalf("expected RESP array header for ACL GETUSER, got %q", header)
	}
	count, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "*")))
	if err != nil {
		t.Fatalf("invalid RESP array length %q: %v", header, err)
	}

	result := make(map[string][]string)
	if count <= 0 {
		return result
	}
	if count%2 != 0 {
		t.Fatalf("expected an even number of elements in ACL GETUSER response, got %d", count)
	}

	for i := 0; i < count; i += 2 {
		nameRaw := readBulkString(t, r)
		parts := strings.SplitN(nameRaw, "\r\n", 3)
		var propName string
		if len(parts) >= 2 {
			propName = parts[1]
		}

		value := listReadRESPArray(t, r)
		if value == nil {
			value = []string{}
		}
		result[propName] = value
	}
	return result
}

// aclContains reports whether needle is present in haystack.
func aclContains(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

// aclReadSimpleError reads a RESP simple error line (e.g. "-WRONGPASS ...\r\n")
// and returns the message with the leading '-' and trailing CRLF stripped.
func aclReadSimpleError(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	line := readLine(t, r)
	if !strings.HasPrefix(line, "-") {
		t.Fatalf("expected RESP simple error, got %q", line)
	}
	return strings.TrimSuffix(strings.TrimPrefix(line, "-"), "\r\n")
}

// aclSHA256Hex computes the lowercase hex-encoded SHA-256 hash of password,
// mirroring how the server is expected to store ACL passwords.
func aclSHA256Hex(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

// --- Stage 1 (doc 108): ACL WHOAMI ---

func TestACLWhoAmiReturnsDefault_Stage01ACLWhoAmi(t *testing.T) {
	requireACLAuthStage(t, 1)
	// Scenario: a fresh connection is auto-authenticated as "default"; ACL WHOAMI
	// should report that username as a RESP bulk string.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "ACL", "WHOAMI")
	if got := readBulkString(t, r); got != "$7\r\ndefault\r\n" {
		t.Fatalf("ACL WHOAMI: expected default bulk string, got %q", got)
	}
}

// --- Stage 2 (doc 109): ACL GETUSER (flags only, hardcoded empty) ---

func TestACLGetUserFlagsEmpty_Stage02ACLGetUserFlags(t *testing.T) {
	requireACLAuthStage(t, 2)
	// Scenario: at this stage the default user's flags are hardcoded to an
	// empty array since nothing has been reported yet.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "ACL", "GETUSER", "default")
	m := aclReadGetUserResponse(t, r)
	if len(m["flags"]) != 0 {
		t.Fatalf("ACL GETUSER flags: expected empty, got %v", m["flags"])
	}
}

// --- Stage 3 (doc 110): the nopass flag ---

func TestACLGetUserNoPassFlag_Stage03NoPassFlag(t *testing.T) {
	requireACLAuthStage(t, 3)
	// Scenario: a fresh default user has the "nopass" flag set, which is why
	// new connections are auto-authenticated.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "ACL", "GETUSER", "default")
	m := aclReadGetUserResponse(t, r)
	if !sliceEqual(m["flags"], []string{"nopass"}) {
		t.Fatalf("ACL GETUSER flags: expected [nopass], got %v", m["flags"])
	}
}

// --- Stage 4 (doc 111): the passwords property ---

func TestACLGetUserPasswordsProperty_Stage04PasswordsProperty(t *testing.T) {
	requireACLAuthStage(t, 4)
	// Scenario: the default user reports nopass in flags and an empty
	// passwords array since no password has ever been configured.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "ACL", "GETUSER", "default")
	m := aclReadGetUserResponse(t, r)
	if !sliceEqual(m["flags"], []string{"nopass"}) {
		t.Fatalf("ACL GETUSER flags: expected [nopass], got %v", m["flags"])
	}
	if len(m["passwords"]) != 0 {
		t.Fatalf("ACL GETUSER passwords: expected empty, got %v", m["passwords"])
	}
}

// --- Stage 5 (doc 112): setting default user password ---

func TestACLSetUserPasswordUpdatesFlags_Stage05SetUserPassword(t *testing.T) {
	requireACLAuthStage(t, 5)
	// Scenario: setting a password for default via ACL SETUSER removes the
	// nopass flag and stores the SHA-256 hash of the password.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	// Baseline regression check (mirrors stage 4).
	writeRESPArray(t, conn, "ACL", "GETUSER", "default")
	m := aclReadGetUserResponse(t, r)
	if !sliceEqual(m["flags"], []string{"nopass"}) {
		t.Fatalf("baseline ACL GETUSER flags: expected [nopass], got %v", m["flags"])
	}
	if len(m["passwords"]) != 0 {
		t.Fatalf("baseline ACL GETUSER passwords: expected empty, got %v", m["passwords"])
	}

	writeRESPArray(t, conn, "ACL", "SETUSER", "default", ">mypassword")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("ACL SETUSER: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "ACL", "GETUSER", "default")
	m = aclReadGetUserResponse(t, r)
	if aclContains(m["flags"], "nopass") {
		t.Fatalf("ACL GETUSER flags: expected nopass to be removed, got %v", m["flags"])
	}
	wantHash := aclSHA256Hex("mypassword")
	if !sliceEqual(m["passwords"], []string{wantHash}) {
		t.Fatalf("ACL GETUSER passwords: expected [%s], got %v", wantHash, m["passwords"])
	}
}

// --- Stage 6 (doc 113): the AUTH command ---

func TestAuthCommandWrongAndCorrectPassword_Stage06AuthCommand(t *testing.T) {
	requireACLAuthStage(t, 6)
	// Scenario: after setting a password, AUTH with the wrong password fails
	// with a WRONGPASS error, and AUTH with the correct password succeeds.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "ACL", "SETUSER", "default", ">mypassword")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("ACL SETUSER: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "AUTH", "default", "wrongpassword")
	errMsg := aclReadSimpleError(t, r)
	if !strings.HasPrefix(errMsg, "WRONGPASS") {
		t.Fatalf("AUTH wrong password: expected error starting with WRONGPASS, got %q", errMsg)
	}

	writeRESPArray(t, conn, "AUTH", "default", "mypassword")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("AUTH correct password: expected +OK, got %q", got)
	}
}

// --- Stage 7 (doc 114): enforce authentication ---

func TestEnforceAuthenticationRejectsUnauthenticatedConnection_Stage07EnforceAuth(t *testing.T) {
	requireACLAuthStage(t, 7)
	// Scenario: once a password is set for default, already-authenticated
	// connections stay logged in, but brand new connections start out
	// unauthenticated and get NOAUTH on any command.
	sp := startTinyRed(t)

	conn1, r1 := dialClient(t, sp)
	writeRESPArray(t, conn1, "ACL", "SETUSER", "default", ">newpassword")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("ACL SETUSER: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn1, "ACL", "WHOAMI")
	if got := readBulkString(t, r1); got != "$7\r\ndefault\r\n" {
		t.Fatalf("ACL WHOAMI on already-authenticated conn: expected default, got %q", got)
	}

	// Fresh connection opened after the password was set.
	conn2, r2 := dialClient(t, sp)
	writeRESPArray(t, conn2, "ACL", "WHOAMI")
	errMsg := aclReadSimpleError(t, r2)
	if !strings.HasPrefix(errMsg, "NOAUTH") {
		t.Fatalf("ACL WHOAMI on unauthenticated conn: expected error starting with NOAUTH, got %q", errMsg)
	}
}

// --- Stage 8 (doc 115): authenticate using AUTH ---

func TestAuthenticateUsingAuthAllowsCommands_Stage08AuthenticateUsingAuth(t *testing.T) {
	requireACLAuthStage(t, 8)
	// Scenario: a fresh, unauthenticated connection gets NOAUTH on any command
	// (not just ACL commands), but after a successful AUTH it can run commands
	// normally for the rest of that connection.
	sp := startTinyRed(t)

	conn1, r1 := dialClient(t, sp)
	writeRESPArray(t, conn1, "ACL", "SETUSER", "default", ">newpassword")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("ACL SETUSER: expected +OK, got %q", got)
	}

	conn2, r2 := dialClient(t, sp)
	writeRESPArray(t, conn2, "PING")
	errMsg := aclReadSimpleError(t, r2)
	if !strings.HasPrefix(errMsg, "NOAUTH") {
		t.Fatalf("PING before AUTH: expected error starting with NOAUTH, got %q", errMsg)
	}

	writeRESPArray(t, conn2, "AUTH", "default", "newpassword")
	if got := readLine(t, r2); got != "+OK\r\n" {
		t.Fatalf("AUTH: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn2, "PING")
	if got := readLine(t, r2); got != "+PONG\r\n" {
		t.Fatalf("PING after AUTH: expected +PONG, got %q", got)
	}
}
