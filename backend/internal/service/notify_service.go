package service

import (
	"bytes"
	"log"
	"net/http"
	"time"

	"oemrpa/internal/model"
	"oemrpa/internal/repository"
)

// maxNotifyFailTimes 通知下游最大失败次数，超限放弃（status=2）
const maxNotifyFailTimes = 10

// NotifyService 下游通知重试服务：
// 通知下游失败时 Enqueue 落库（notify_record），定时任务 RetryDue 按指数退避补推，成功置 1、超限置 2 放弃。
type NotifyService struct {
	repo   *repository.NotifyRecordRepository
	client *http.Client
}

func NewNotifyService(repo *repository.NotifyRecordRepository) *NotifyService {
	return &NotifyService{
		repo:   repo,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Enqueue 通知失败后落库待重推（尽力而为，落库失败仅记录日志）
func (s *NotifyService) Enqueue(bizType string, recordID, userID int64, targetURL, payload string) {
	rec := &model.NotifyRecord{
		BizType:   bizType,
		RecordID:  recordID,
		UserID:    userID,
		TargetURL: targetURL,
		Payload:   payload,
		Status:    model.NotifyPending,
		FailTimes: 0,
	}
	if err := s.repo.Create(rec); err != nil {
		log.Printf("落库待重推通知失败 [biz_type=%s, record_id=%d]: %v", bizType, recordID, err)
	}
}

// RetryDue 重推所有到期待推通知（定时任务调用）
func (s *NotifyService) RetryDue() {
	records, err := s.repo.ListRetryable(time.Now(), 100)
	if err != nil {
		log.Printf("查询待重推通知失败: %v", err)
		return
	}
	for _, rec := range records {
		s.retryOne(rec)
	}
}

// retryOne 重推单条通知
func (s *NotifyService) retryOne(rec *model.NotifyRecord) {
	resp, err := s.client.Post(rec.TargetURL, "application/json", bytes.NewReader([]byte(rec.Payload)))
	now := time.Now()
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if uerr := s.repo.UpdateResult(rec.ID, model.NotifySuccess, rec.FailTimes, "", nil); uerr != nil {
				log.Printf("标记通知成功失败 [id=%d]: %v", rec.ID, uerr)
			}
			log.Printf("补推通知成功 [id=%d, biz_type=%s, record_id=%d]", rec.ID, rec.BizType, rec.RecordID)
			return
		}
		err = errStatusCode(resp.StatusCode)
	}

	failTimes := rec.FailTimes + 1
	status := model.NotifyPending
	var next *time.Time
	if failTimes >= maxNotifyFailTimes {
		status = model.NotifyAbandoned
	} else {
		// 指数退避：1/2/4/8/16/32/60... 分钟（封顶 60 分钟）
		backoff := 1 << uint(failTimes)
		if backoff > 60 {
			backoff = 60
		}
		t := now.Add(time.Duration(backoff) * time.Minute)
		next = &t
	}
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	if uerr := s.repo.UpdateResult(rec.ID, status, failTimes, errMsg, next); uerr != nil {
		log.Printf("更新通知重推状态失败 [id=%d]: %v", rec.ID, uerr)
	}
	log.Printf("补推通知失败 [id=%d, biz_type=%s, record_id=%d, fail_times=%d, status=%d, err=%v]",
		rec.ID, rec.BizType, rec.RecordID, failTimes, status, err)
}

type statusCodeError int

func (e statusCodeError) Error() string { return "下游返回非 2xx" }
func errStatusCode(code int) error      { return statusCodeError(code) }
