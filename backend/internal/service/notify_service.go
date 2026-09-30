package service

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"oemrpa/internal/model"
	"oemrpa/internal/repository"
)

// maxNotifyFailTimes 通知下游最大失败次数，超限放弃（status=2）
const maxNotifyFailTimes = 10

// NotifyService 下游通知重试服务：
// 通知下游失败时 Enqueue 落库（notify_record，以业务类型 + 业务单号为主键），定时任务 RetryDue 按指数退避补推，
// 推送成功后直接删除该行、超限置 2 放弃（保留供排障）。后台可手动重推单条（RetryNow）；
// 同一记录以进程内互斥串行，避免与定时任务并发双发。
type NotifyService struct {
	repo   *repository.NotifyRecordRepository
	client *http.Client
	locks  sync.Map // key: biz_type|biz_no → *sync.Mutex
}

func NewNotifyService(repo *repository.NotifyRecordRepository) *NotifyService {
	return &NotifyService{
		repo:   repo,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Enqueue 通知失败后落库待重推（尽力而为，落库失败仅记录日志）。
// bizNo 为该业务的单号（无单号的实体用其业务内唯一标识）：同一 (bizType, bizNo) 重复入队即重置重试进度。
func (s *NotifyService) Enqueue(bizType, bizNo string, recordID, userID int64, targetURL, payload string) {
	if bizNo == "" {
		log.Printf("落库待重推通知缺少业务单号，跳过 [biz_type=%s, record_id=%d]", bizType, recordID)
		return
	}
	rec := &model.NotifyRecord{
		BizType:   bizType,
		BizNo:     bizNo,
		RecordID:  recordID,
		UserID:    userID,
		TargetURL: targetURL,
		Payload:   payload,
		Status:    model.NotifyPending,
		FailTimes: 0,
	}
	if err := s.repo.Upsert(rec); err != nil {
		log.Printf("落库待重推通知失败 [biz_type=%s, biz_no=%s, record_id=%d]: %v", bizType, bizNo, recordID, err)
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
		s.retryOnce(rec)
	}
}

// RetryNow 后台手动立即重推指定通知：已放弃的需 force=true（先重置为待推再推）。
// 返回重推后记录是否已被删除（即推送成功）。
func (s *NotifyService) RetryNow(bizType, bizNo string, force bool) (bool, error) {
	rec, err := s.repo.Get(bizType, bizNo)
	if err != nil {
		return false, err
	}
	if rec == nil {
		return false, errors.New("通知记录不存在")
	}
	if rec.Status == model.NotifyAbandoned {
		if !force {
			return false, errors.New("该通知已放弃重推，需强制重推")
		}
		if rerr := s.repo.ResetForRetry(bizType, bizNo); rerr != nil {
			return false, rerr
		}
		rec.Status = model.NotifyPending
		rec.FailTimes = 0
	}
	s.retryOnce(rec)

	latest, lerr := s.repo.Get(bizType, bizNo)
	if lerr != nil {
		return false, lerr
	}
	if latest != nil {
		// 推送未成功：该行仍在（失败原因已写入 last_error，见「最后错误」列）
		return false, errors.New("重推未成功，失败原因已更新")
	}
	// 推送成功后该行已删除
	return true, nil
}

// retryOnce 加锁重推单条（定时任务与手动重推共用，同一记录串行执行）
func (s *NotifyService) retryOnce(rec *model.NotifyRecord) {
	v, _ := s.locks.LoadOrStore(rec.BizType+"|"+rec.BizNo, &sync.Mutex{})
	mu, _ := v.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	s.retryOne(rec)
}

// retryOne 重推单条通知：成功即删除该行；失败累计次数（超限置 2 放弃，保留供排障）
func (s *NotifyService) retryOne(rec *model.NotifyRecord) {
	resp, err := s.client.Post(rec.TargetURL, "application/json", bytes.NewReader([]byte(rec.Payload)))
	now := time.Now()
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// 推送成功即删除该行（成功历史不落表，留痕见 syscall.log / business.log）
			if derr := s.repo.Delete(rec.BizType, rec.BizNo); derr != nil {
				log.Printf("删除已送达通知失败 [biz_type=%s, biz_no=%s]: %v", rec.BizType, rec.BizNo, derr)
			}
			log.Printf("补推通知成功 [biz_type=%s, biz_no=%s, record_id=%d]", rec.BizType, rec.BizNo, rec.RecordID)
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
	if uerr := s.repo.UpdateResult(rec.BizType, rec.BizNo, status, failTimes, errMsg, next); uerr != nil {
		log.Printf("更新通知重推状态失败 [biz_type=%s, biz_no=%s]: %v", rec.BizType, rec.BizNo, uerr)
	}
	log.Printf("补推通知失败 [biz_type=%s, biz_no=%s, record_id=%d, fail_times=%d, status=%d, err=%v]",
		rec.BizType, rec.BizNo, rec.RecordID, failTimes, status, err)
}

type statusCodeError int

func (e statusCodeError) Error() string { return "下游返回非 2xx" }
func errStatusCode(code int) error      { return statusCodeError(code) }
