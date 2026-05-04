package dbw

import (
	"sync"
	"sync/atomic"
	"time"
)

var snowflakeMachineId atomic.Int64

func init() {
	snowflakeMachineId.Store(1)
}

// SetSnowflakeMachineId sets the machine ID for the Snowflake ID generator.
func SetSnowflakeMachineId(id int64) {
	snowflakeMachineId.Store(id)
}

// Snowflake is a distributed unique ID generator using the Snowflake algorithm.
type Snowflake struct {
	sync.Mutex
	timestamp int64
	machineId int64
	sequence  int64
}

// CreateSnowflakeFactory creates a new Snowflake instance with the configured machine ID.
func CreateSnowflakeFactory() Snowflake {
	return Snowflake{
		machineId: snowflakeMachineId.Load(),
	}
}

var (
	snowFlakeMu sync.RWMutex
	snowFlake   *Snowflake
)

// GetSnowflake returns the singleton Snowflake instance (thread-safe, lazy initialization).
func GetSnowflake() *Snowflake {
	snowFlakeMu.RLock()
	if snowFlake != nil {
		snowFlakeMu.RUnlock()
		return snowFlake
	}
	snowFlakeMu.RUnlock()

	snowFlakeMu.Lock()
	defer snowFlakeMu.Unlock()
	if snowFlake == nil {
		s := CreateSnowflakeFactory()
		snowFlake = &s
	}
	return snowFlake
}

// GetId generates and returns a unique Snowflake ID.
func (s *Snowflake) GetId() int64 {
	s.Lock()
	defer s.Unlock()
	now := time.Now().UnixNano() / 1e6
	if s.timestamp == now {
		s.sequence = (s.sequence + 1) & 0xFFF
		if s.sequence == 0 {
			for now <= s.timestamp {
				now = time.Now().UnixNano() / 1e6
			}
		}
	} else {
		s.sequence = 0
	}
	s.timestamp = now
	r := (now-1483228800000)<<22 | (s.machineId << 12) | s.sequence
	return r
}
