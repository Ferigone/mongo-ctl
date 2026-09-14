package mongoconf

import (
	"strings"
	"testing"
)

// The stock mongod.cfg shipped by the MongoDB Windows installer.
const stockConfig = `# mongod.conf

# for documentation of all options, see:
#   http://docs.mongodb.org/manual/reference/configuration-options/

# Where and how to store data.
storage:
  dbPath: C:\Program Files\MongoDB\Server\8.3\data

# where to write logging data.
systemLog:
  destination: file
  logAppend: true
  path: C:\Program Files\MongoDB\Server\8.3\log\mongod.log

# network interfaces
net:
  port: 27017
  bindIp: 127.0.0.1
`

func TestParsePreservesCommentsOnEdit(t *testing.T) {
	doc, err := Parse([]byte(stockConfig))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if err := doc.SetPort(27020); err != nil {
		t.Fatalf("SetPort: %v", err)
	}

	rendered := doc.String()

	for _, comment := range []string{
		"# mongod.conf",
		"# Where and how to store data.",
		"# where to write logging data.",
		"# network interfaces",
	} {
		if !strings.Contains(rendered, comment) {
			t.Errorf("comment %q was lost on round-trip:\n%s", comment, rendered)
		}
	}

	if got := doc.Port(); got != 27020 {
		t.Errorf("Port() = %d, want 27020", got)
	}
	if strings.Contains(rendered, "27017") {
		t.Errorf("old port still present:\n%s", rendered)
	}
}

func TestRoundTripPreservesUntouchedKeys(t *testing.T) {
	doc, err := Parse([]byte(stockConfig))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := doc.SetDBPath(`D:\MongoData\demo`); err != nil {
		t.Fatalf("SetDBPath: %v", err)
	}

	rendered := doc.String()

	if !strings.Contains(rendered, "logAppend: true") {
		t.Errorf("untouched key logAppend was lost:\n%s", rendered)
	}
	if got := doc.LogDestination(); got != "file" {
		t.Errorf("LogDestination() = %q, want %q", got, "file")
	}
	if got := doc.DBPath(); got != `D:\MongoData\demo` {
		t.Errorf("DBPath() = %q", got)
	}
}

func TestSetCreatesIntermediateMappings(t *testing.T) {
	doc := New()
	if err := doc.SetReplSetName("rs-local"); err != nil {
		t.Fatalf("SetReplSetName: %v", err)
	}
	if err := doc.SetOplogSizeMB(128); err != nil {
		t.Fatalf("SetOplogSizeMB: %v", err)
	}

	if got := doc.ReplSetName(); got != "rs-local" {
		t.Errorf("ReplSetName() = %q, want %q", got, "rs-local")
	}

	reparsed, err := Parse([]byte(doc.String()))
	if err != nil {
		t.Fatalf("re-parse rendered document: %v", err)
	}
	if got := reparsed.ReplSetName(); got != "rs-local" {
		t.Errorf("after re-parse ReplSetName() = %q", got)
	}
	if got := reparsed.GetInt(pathOplogSizeMB...); got != 128 {
		t.Errorf("after re-parse oplogSizeMB = %d, want 128", got)
	}
}

func TestClearReplSetNameReturnsToStandalone(t *testing.T) {
	doc := ForInstance(`D:\MongoData\demo`, 27018)
	if err := doc.SetReplSetName("rs-local"); err != nil {
		t.Fatalf("SetReplSetName: %v", err)
	}

	doc.ClearReplSetName()

	if got := doc.ReplSetName(); got != "" {
		t.Errorf("ReplSetName() = %q, want empty", got)
	}
	if strings.Contains(doc.String(), "replication") {
		t.Errorf("replication section survived:\n%s", doc.String())
	}
	if got := doc.Port(); got != 27018 {
		t.Errorf("clearing replication disturbed port: got %d", got)
	}
}

func TestForInstanceOmitsSystemLog(t *testing.T) {
	doc := ForInstance(`D:\MongoData\demo`, 27018)

	if got := doc.LogDestination(); got != "" {
		t.Errorf("LogDestination() = %q; supervisor needs mongod on stdout", got)
	}
	if got := doc.BindIP(); got != "127.0.0.1" {
		t.Errorf("BindIP() = %q, want loopback", got)
	}
}

func TestParseEmptyInput(t *testing.T) {
	doc, err := Parse([]byte("   \n"))
	if err != nil {
		t.Fatalf("Parse of blank input: %v", err)
	}
	if err := doc.SetPort(27017); err != nil {
		t.Fatalf("SetPort on empty doc: %v", err)
	}
	if got := doc.Port(); got != 27017 {
		t.Errorf("Port() = %d", got)
	}
}

func TestParseRejectsNonMapping(t *testing.T) {
	if _, err := Parse([]byte("- one\n- two\n")); err == nil {
		t.Error("expected error for a YAML sequence document")
	}
}
