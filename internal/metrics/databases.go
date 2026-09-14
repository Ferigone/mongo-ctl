package metrics

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mongoctl/internal/mongoclient"
)

// DatabaseInfo describes one database on a server.
type DatabaseInfo struct {
	Name        string  `json:"name"`
	SizeOnDisk  int64   `json:"sizeOnDisk"`
	Empty       bool    `json:"empty"`
	Collections int     `json:"collections"`
	Objects     int64   `json:"objects"`
	DataSize    int64   `json:"dataSize"`
	StorageSize int64   `json:"storageSize"`
	IndexSize   int64   `json:"indexSize"`
	AvgObjSize  float64 `json:"avgObjSize"`
}

type dbStats struct {
	Collections int     `bson:"collections"`
	Objects     int64   `bson:"objects"`
	DataSize    int64   `bson:"dataSize"`
	StorageSize int64   `bson:"storageSize"`
	IndexSize   int64   `bson:"indexSize"`
	AvgObjSize  float64 `bson:"avgObjSize"`
}

// Databases lists every database on a server with its storage statistics.
// It is heavier than a serverStatus poll, so the UI refreshes it on demand
// rather than on the metrics tick.
func Databases(ctx context.Context, port int) ([]DatabaseInfo, error) {
	client, err := mongoclient.Connect(port, mongoclient.DefaultTimeout)
	if err != nil {
		return nil, err
	}
	defer client.Disconnect(context.Background())

	listing, err := client.ListDatabases(ctx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("list databases on port %d: %w", port, err)
	}

	databases := make([]DatabaseInfo, 0, len(listing.Databases))
	for _, spec := range listing.Databases {
		info := DatabaseInfo{
			Name:       spec.Name,
			SizeOnDisk: spec.SizeOnDisk,
			Empty:      spec.Empty,
		}

		var stats dbStats
		command := bson.D{{Key: "dbStats", Value: 1}}
		if err := client.Database(spec.Name).RunCommand(ctx, command).Decode(&stats); err == nil {
			info.Collections = stats.Collections
			info.Objects = stats.Objects
			info.DataSize = stats.DataSize
			info.StorageSize = stats.StorageSize
			info.IndexSize = stats.IndexSize
			info.AvgObjSize = stats.AvgObjSize
		}

		databases = append(databases, info)
	}
	return databases, nil
}
