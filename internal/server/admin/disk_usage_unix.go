//go:build unix

package admin

import (
	"math"

	"golang.org/x/sys/unix"
)

func filesystemUsage(path string) (available int64, total int64, err error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}

	blockSize := int64(stat.Bsize)
	if blockSize <= 0 {
		return 0, 0, nil
	}

	return statfsBytes(stat.Bavail, blockSize), statfsBytes(stat.Blocks, blockSize), nil
}

func statfsBytes(blocks uint64, blockSize int64) int64 {
	if blockSize <= 0 {
		return 0
	}

	unsignedBlockSize := uint64(blockSize)
	if blocks > uint64(math.MaxInt64)/unsignedBlockSize {
		return math.MaxInt64
	}
	bytes := blocks * unsignedBlockSize
	// #nosec G115 -- bytes is bounded above by math.MaxInt64 before conversion.
	return int64(bytes)
}
