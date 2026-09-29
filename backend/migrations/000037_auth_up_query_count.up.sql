-- 000037 认证订单新增 up_query_count（已向上游 get_result 查询次数）
-- 背景：上游 FinAuth get_result 接口「调用信息保存有效期为一天，且仅支持 3 次调用，
--       第 4 次或超过有效期调用则返回错误」，错误码 403 DATA_DESTROYED（超过可查询时间或超过最多可查询次数）。
--       此前下游轮询（/v1/fv/result）、用户回跳（/v1/fv/return）、5 分钟定时任务、取图（/v1/fv/best-img）
--       四条链路都会无次数上限地调用该接口，往往在用户还没完成核身时就把 3 次额度耗尽，
--       第 4 次返回 DATA_DESTROYED 被当成「认证失败」，订单被误判为失败并退款。
-- 处理：新增 up_query_count 记录已查询次数，服务层以 model.UpstreamQueryLimit(3) 为硬上限做预算，
--       仅保留「用户回跳校对」等必要查询；存量进行中订单标记为已消耗 1 次（发布前链路已在主动查询）。
-- 说明：禁止在迁移内 USE 切换会话库；跨库操作一律全限定库名.表名。幂等。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_record');

SET @has_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = 'oem_fv' AND TABLE_NAME = 'auth_record' AND COLUMN_NAME = 'up_query_count');
SET @sql_add = IF(@has_tbl > 0 AND @has_col = 0,
    'ALTER TABLE `oem_fv`.`auth_record`
        ADD COLUMN `up_query_count` int NOT NULL DEFAULT 0 COMMENT ''已向上游 get_result 查询次数（上游每单仅允许 3 次）'' AFTER `up_request_id`',
    'SELECT 1');
PREPARE stmt_add FROM @sql_add; EXECUTE stmt_add; DEALLOCATE PREPARE stmt_add;

-- 存量仍进行中的订单：发布前链路已在轮询/定时任务中主动查询过上游，统一标记为已消耗 1 次，
-- 避免新逻辑上线后再连续查询把剩余额度打满（数据一旦被上游销毁则结果永久不可取回）。
UPDATE `oem_fv`.`auth_record`
SET up_query_count = 1, updated_at = updated_at
WHERE status IN (0, 1) AND COALESCE(up_query_count, 0) = 0;
