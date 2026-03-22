package dbw

import (
	"sync"
	"time"
)

var snowflakeMachineId int64 = 1

func SetSnowflakeMachineId(id int64) {
	snowflakeMachineId = id
}

// CreateSnowflakeFactory 创建一个雪花算法生成器(生成工厂)
func CreateSnowflakeFactory() Snowflake {
	return Snowflake{
		timestamp: 0,
		machineId: snowflakeMachineId,
		sequence:  0,
	}
}
func GetSnowflake() *Snowflake {
	if snowFlake == nil {
		s := CreateSnowflakeFactory()
		snowFlake = &s
	}
	return snowFlake
}

type Snowflake struct {
	sync.Mutex
	timestamp int64
	machineId int64
	sequence  int64
}

// GetId 生成分布式ID
func (s *Snowflake) GetId() int64 {
	s.Lock()
	defer func() {
		s.Unlock()
	}()
	now := time.Now().UnixNano() / 1e6
	if s.timestamp == now {
		s.sequence = (s.sequence + 1) & int64(-1^(-1<<uint(12)))
		if s.sequence == 0 {
			for now <= s.timestamp {
				now = time.Now().UnixNano() / 1e6
			}
		}
	} else {
		s.sequence = 0
	}
	s.timestamp = now
	r := (now-int64(1483228800000))<<(uint(12)+uint(10)) | (s.machineId << uint(12)) | (s.sequence)
	return r
}
