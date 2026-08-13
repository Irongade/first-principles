package config

import (
	"fmt"
	"io/fs"
	"kvstore/constants"
	"kvstore/types"
	"log/slog"
	"os"
	"strconv"
	"time"
)

// Defaults mirror the values in the CreateDefault*Config() constructors.
// Those stay pure and environment-unaware; this is the only place the
// process looks at os.Getenv.
const (
	DefaultDataDir  = "data"
	DefaultFileName = "text.txt"
	DefaultVersion  = "V1"

	DefaultMaxSegmentSize = 1 << 20  // 1 MiB
	DefaultBufferSize     = 32 << 10 // 32 KiB
	DefaultMaxRecordSize  = 4 << 20  // 4 MiB

	DefaultSyncPolicy = constants.SyncEveryWrite

	DefaultStaleSegmentThreshold = 5
	DefaultCompactionInterval    = 5 * time.Minute
	DefaultEnableCompaction      = true

	DefaultStorageFormat = constants.SegmentFileFormat
	DefaultIndexFormat   = constants.PositionIndex
	DefaultFormatter     = constants.TEXT_FORMATTER

	DefaultFilePermission = fs.FileMode(0o600)
	DefaultDirPermission  = fs.FileMode(0o755)
)

type Config struct {
	DataDir  string
	FileName string
	Version  string

	MaxSegmentSize        int
	BufferSize            int
	StaleSegmentThreshold int
	MaxRecordSize         int

	SyncPolicy types.SyncPolicy

	EnableCompaction   bool
	CompactionInterval time.Duration

	StorageFormat types.FileFormat
	IndexFormat   types.IndexFormat
	Formatter     types.FormatterOptions

	FilePermission      fs.FileMode
	DirectoryPermission fs.FileMode
}

func Load(log *slog.Logger) Config {
	return Config{
		DataDir:        getString(log, "STORE_DATA_DIR", DefaultDataDir),
		FileName:       getString(log, "STORE_FILENAME", DefaultDataDir),
		Version:        getString(log, "STORE_VERSION", DefaultDataDir),
		MaxSegmentSize: getInt(log, "STORE_MAX_SEGMENT_SIZE", DefaultMaxSegmentSize),
		BufferSize:     getInt(log, "STORE_BUFFER_SIZE", DefaultBufferSize),
		MaxRecordSize:  getInt(log, "STORE_MAX_RECORD_SIZE", DefaultMaxRecordSize),

		StaleSegmentThreshold: getInt(log, "STORE_STALE_SEGMENT_THRESHOLD", DefaultStaleSegmentThreshold),
		EnableCompaction:      getBool(log, "STORE_ENABLE_COMPACTION", DefaultEnableCompaction),
		CompactionInterval:    getDuration(log, "STORE_COMPACTION_INTERVAL", DefaultCompactionInterval),
		StorageFormat: getEnum(log, "STORE_STORAGE_FORMAT", DefaultStorageFormat,
			map[string]types.FileFormat{
				"segment": constants.SegmentFileFormat,
				"simple":  constants.SimpleFileFormat,
			}),
		IndexFormat: getEnum(log, "STORE_INDEX_FORMAT", DefaultIndexFormat,
			map[string]types.IndexFormat{
				"value":    constants.ValueIndex,
				"position": constants.PositionIndex,
			}),
		SyncPolicy: getEnum(log, "STORE_SYNC_POLICY", DefaultSyncPolicy,
			map[string]types.SyncPolicy{
				"never":       constants.SyncNever,
				"every-write": constants.SyncEveryWrite,
			}),
		FilePermission:      DefaultFilePermission, // not configurable for now
		DirectoryPermission: DefaultDirPermission,
	}
}

func getString(log *slog.Logger, name, fallback string) string {
	raw, ok := os.LookupEnv(name)
	if !ok || raw == "" {
		return fallback
	}
	return raw
}

func getInt(log *slog.Logger, name string, fallback int) int {
	raw, ok := os.LookupEnv(name)
	if !ok || raw == "" {
		return fallback
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		log.Warn("invalid config value, using default",
			"variable", name, "value", raw, "default", fallback, "error", err)
		return fallback
	}
	if n <= 0 {
		log.Warn("config value must be positive, using default",
			"variable", name, "value", n, "default", fallback)
		return fallback
	}
	return n
}

func getDuration(log *slog.Logger, name string, fallback time.Duration) time.Duration {
	raw, ok := os.LookupEnv(name)
	if !ok || raw == "" {
		return fallback
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		log.Warn("invalid duration, using default",
			"variable", name, "value", raw, "default", fallback, "error", err)
		return fallback
	}
	if d <= 0 {
		log.Warn("duration must be positive, using default",
			"variable", name, "value", d, "default", fallback)
		return fallback
	}
	return d
}

func getEnum[T comparable](log *slog.Logger, name string, fallback T, accepted map[string]T) T {
	raw, ok := os.LookupEnv(name)
	if !ok || raw == "" {
		return fallback
	}

	value, valid := accepted[raw]
	if !valid {
		log.Warn("unrecognised value, using default",
			"variable", name, "value", raw, "default", fallback,
			"accepted", keys(accepted))
		return fallback
	}
	return value
}

func getBool(logger *slog.Logger, name string, fallback bool) bool {
	raw, ok := os.LookupEnv(name)
	if !ok || raw == "" {
		return fallback
	}

	b, err := strconv.ParseBool(raw)
	if err != nil {
		logger.Warn("invalid bool, using default",
			"variable", name, "value", raw, "default", fallback)
		return fallback
	}
	return b
}

func keys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func (c Config) LogSummary(logger *slog.Logger) {
	logger.Info("storage configuration",
		"data_dir", c.DataDir,
		"filename", c.FileName,
		"max_segment_size", c.MaxSegmentSize,
		"buffer_size", c.BufferSize,
		"max_record_size", c.MaxRecordSize,
		"sync_policy", fmt.Sprint(c.SyncPolicy),
		"stale_segment_threshold", c.StaleSegmentThreshold,
		"compaction_interval", c.CompactionInterval,
		"storage_format", fmt.Sprint(c.StorageFormat),
		"index_format", fmt.Sprint(c.IndexFormat),
		"formatter", fmt.Sprint(c.Formatter),
		"file_permission", fmt.Sprintf("%#o", c.FilePermission),
		"directory_permission", fmt.Sprintf("%#o", c.DirectoryPermission),
	)
}

func (c Config) Validate(log *slog.Logger) {
	if c.BufferSize >= c.MaxSegmentSize {
		log.Warn("buffer size >= segment size; segments will overshoot their threshold",
			"buffer_size", c.BufferSize, "max_segment_size", c.MaxSegmentSize)
	}
	if c.IndexFormat != constants.PositionIndex {
		log.Warn("compaction is disabled: it requires the position index",
			"index_format", fmt.Sprint(c.IndexFormat))
	}
}
