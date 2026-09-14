package metrics

// Keys inside the wiredTiger.cache document that mongod reports.
const (
	cacheUsedKey = "bytes currently in the cache"
	cacheMaxKey  = "maximum bytes configured"
)

// serverStatus decodes the subset of the serverStatus command this app charts.
type serverStatus struct {
	Uptime      float64         `bson:"uptime"`
	Connections connectionStats `bson:"connections"`
	Mem         memStats        `bson:"mem"`
	Opcounters  opcounters      `bson:"opcounters"`
	Network     networkStats    `bson:"network"`
	OpLatencies opLatencies     `bson:"opLatencies"`
	GlobalLock  globalLock      `bson:"globalLock"`

	// WiredTiger is absent when the server runs a different storage engine.
	WiredTiger *wiredTigerStats `bson:"wiredTiger"`
}

type connectionStats struct {
	Current   int `bson:"current"`
	Active    int `bson:"active"`
	Available int `bson:"available"`
}

type memStats struct {
	Resident int `bson:"resident"`
	Virtual  int `bson:"virtual"`
}

type opcounters struct {
	Insert  int64 `bson:"insert"`
	Query   int64 `bson:"query"`
	Update  int64 `bson:"update"`
	Delete  int64 `bson:"delete"`
	GetMore int64 `bson:"getmore"`
	Command int64 `bson:"command"`
}

type networkStats struct {
	BytesIn     int64 `bson:"bytesIn"`
	BytesOut    int64 `bson:"bytesOut"`
	NumRequests int64 `bson:"numRequests"`
}

type latencyStat struct {
	Latency int64 `bson:"latency"`
	Ops     int64 `bson:"ops"`
}

type opLatencies struct {
	Reads    latencyStat `bson:"reads"`
	Writes   latencyStat `bson:"writes"`
	Commands latencyStat `bson:"commands"`
}

type globalLock struct {
	CurrentQueue struct {
		Readers int `bson:"readers"`
		Writers int `bson:"writers"`
	} `bson:"currentQueue"`
}

// wiredTigerStats keeps the cache document untyped: it carries hundreds of
// engine counters whose types vary, and decoding it strictly would break the
// whole poll over a field this app never reads.
type wiredTigerStats struct {
	Cache map[string]any `bson:"cache"`
}

func (s *serverStatus) cacheUsed() int64 { return s.cacheValue(cacheUsedKey) }
func (s *serverStatus) cacheMax() int64  { return s.cacheValue(cacheMaxKey) }

func (s *serverStatus) cacheValue(key string) int64 {
	if s.WiredTiger == nil {
		return 0
	}
	return toInt64(s.WiredTiger.Cache[key])
}

func toInt64(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int32:
		return int64(typed)
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return 0
	}
}
