package service

import (
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

// PreConsumeQuotaForJob pre-consumes quota and returns the amount or error.
func PreConsumeQuotaForJob(userID string, quota int) (int, error) {
	uid, err := strconv.Atoi(userID)
	if err != nil {
		return 0, err
	}
	userQuota, err := model.GetUserQuota(uid, false)
	if err != nil {
		return 0, err
	}
	if userQuota < quota {
		return 0, fmt.Errorf("额度不足，需要 %d，剩余 %d", quota, userQuota)
	}

	err = model.DecreaseUserQuota(uid, quota, false)
	if err != nil {
		return 0, err
	}
	return quota, nil
}

// RefundQuotaForJob refunds pre-consumed quota.
func RefundQuotaForJob(userID string, quota int) {
	if quota <= 0 {
		return
	}
	uid, err := strconv.Atoi(userID)
	if err != nil {
		return
	}
	_ = model.IncreaseUserQuota(uid, quota, false)
	logger.LogInfo(nil, fmt.Sprintf("用户 %d 任务创建失败，退还预扣费额度 %d", uid, quota))
}
