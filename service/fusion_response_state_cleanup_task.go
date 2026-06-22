package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const fusionResponseStateCleanupInterval = 30 * time.Minute

var (
	fusionResponseStateCleanupOnce    sync.Once
	fusionResponseStateCleanupRunning atomic.Bool
)

func StartFusionResponseStateCleanupTask() {
	fusionResponseStateCleanupOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("fusion response state cleanup task started: interval=%s", fusionResponseStateCleanupInterval))
			ticker := time.NewTicker(fusionResponseStateCleanupInterval)
			defer ticker.Stop()

			runFusionResponseStateCleanupOnce()
			for range ticker.C {
				runFusionResponseStateCleanupOnce()
			}
		})
	})
}

func runFusionResponseStateCleanupOnce() {
	if !fusionResponseStateCleanupRunning.CompareAndSwap(false, true) {
		return
	}
	defer fusionResponseStateCleanupRunning.Store(false)

	deleted, err := model.DeleteExpiredFusionResponseStates(time.Now().Unix())
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("fusion response state cleanup failed: %v", err))
		return
	}
	if common.DebugEnabled && deleted > 0 {
		logger.LogDebug(context.Background(), "fusion response state cleanup: deleted=%d", deleted)
	}
}
