package mongoconf

// Well-known mongod.conf key paths.
var (
	pathPort         = []string{"net", "port"}
	pathBindIP       = []string{"net", "bindIp"}
	pathDBPath       = []string{"storage", "dbPath"}
	pathReplSetName  = []string{"replication", "replSetName"}
	pathOplogSizeMB  = []string{"replication", "oplogSizeMB"}
	pathLogDest      = []string{"systemLog", "destination"}
	pathLogPath      = []string{"systemLog", "path"}
	pathAuthorization= []string{"security", "authorization"}
)

// ForInstance builds a fresh standalone configuration bound to loopback.
//
// systemLog is deliberately left unset: the supervisor owns mongod's stdout so
// it can stream logs to the UI, which only works while mongod logs to the console.
func ForInstance(dbPath string, port int) *Doc {
	doc := New()
	_ = doc.SetDBPath(dbPath)
	_ = doc.SetPort(port)
	_ = doc.SetBindIP("127.0.0.1")
	return doc
}

// Port returns the configured TCP port.
func (d *Doc) Port() int { return d.GetInt(pathPort...) }

// SetPort sets the TCP port.
func (d *Doc) SetPort(port int) error { return d.Set(port, pathPort...) }

// BindIP returns the configured bind address list.
func (d *Doc) BindIP() string { return d.GetString(pathBindIP...) }

// SetBindIP sets the bind address list.
func (d *Doc) SetBindIP(addresses string) error { return d.Set(addresses, pathBindIP...) }

// DBPath returns the data directory.
func (d *Doc) DBPath() string { return d.GetString(pathDBPath...) }

// SetDBPath sets the data directory.
func (d *Doc) SetDBPath(dir string) error { return d.Set(dir, pathDBPath...) }

// ReplSetName returns the replica set name, or "" for a standalone server.
func (d *Doc) ReplSetName() string { return d.GetString(pathReplSetName...) }

// SetReplSetName enlists the server in a replica set. mongod only honours this
// at startup, so changing it always requires a restart.
func (d *Doc) SetReplSetName(name string) error { return d.Set(name, pathReplSetName...) }

// ClearReplSetName turns the server back into a standalone on next start. The
// whole replication section goes, since this app is the only thing that writes it.
func (d *Doc) ClearReplSetName() {
	d.Remove("replication")
}

// SetOplogSizeMB caps the oplog, which otherwise defaults to 5% of free disk.
func (d *Doc) SetOplogSizeMB(size int) error { return d.Set(size, pathOplogSizeMB...) }

// LogDestination returns the systemLog destination, or "" when mongod logs to console.
func (d *Doc) LogDestination() string { return d.GetString(pathLogDest...) }

// LogPath returns the systemLog file path, if one is configured.
func (d *Doc) LogPath() string { return d.GetString(pathLogPath...) }

// AuthorizationEnabled reports whether access control is switched on.
func (d *Doc) AuthorizationEnabled() bool { return d.GetString(pathAuthorization...) == "enabled" }
