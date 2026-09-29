-- 000026 清理「下游 API 调用」残留在账户个人实名记录表的历史行
-- 背景：3257414（9-22 19:43 实名记录拆表）之前，人脸核验 API 发起（StartFvAuth）会额外往
--       oem_sys.kyc 插入一条记录（只写 user_id/biz_no/name/id_card/status，标 source=2，
--       不关联认证订单）。该写入已随 3257414 移除，但迁移 000020 只删除了「已关联认证订单」的
--       历史行，未关联订单的残留行仍在：它们不是账户实名，却会被认证订单结果回写逻辑误标为
--       「认证成功」，与账户实名记录混淆。
-- 判定依据：账户实名行由实名页发起，必然带 return_url 与 biz_extra_data
--       （{"type":"user_auth","user_id":N}）；残留行这三列全空且未关联认证记录。
--       核验信息在 oem_fv.auth_record 中完整保留，删除后不丢数据。
-- 注意：禁止在迁移文件内使用 USE 切换会话库。跨库 DDL 一律用反引号全限定库名.表名。
SET @has_tbl = (SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = 'oem_sys' AND TABLE_NAME = 'kyc');

SET @sql_del = IF(@has_tbl > 0,
    'DELETE FROM `oem_sys`.`kyc`
      WHERE COALESCE(auth_record_id, 0) = 0
        AND COALESCE(return_url, '''') = ''''
        AND COALESCE(biz_extra_data, '''') = ''''',
    'SELECT 1');
PREPARE stmt_del FROM @sql_del; EXECUTE stmt_del; DEALLOCATE PREPARE stmt_del;