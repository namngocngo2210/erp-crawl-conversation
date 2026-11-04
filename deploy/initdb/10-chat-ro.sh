#!/bin/sh
# Tạo role chỉ-đọc `chat_ro` — đầu phía crawler của cầu postgres_fdw mà stack
# desi-order dùng để nạp ai.tin_nhan / ai.hoi_thoai.
#
# VÌ SAO PHẢI LÀ INITDB SCRIPT, KHÔNG PHẢI BƯỚC TAY TRONG RUNBOOK:
# 22/09/2026 volume `deploy_pgdata` bị dựng lại từ đầu (PG_VERSION ghi lúc
# 13:20:30 +07). Cluster mới = mất SẠCH role, trong đó có `chat_ro` vốn được tạo
# tay theo runbook 2026-09-04 bước 1 của desi-order. Bảng thì migration của
# crawler tự dựng lại được, role thì KHÔNG có ai dựng lại → mọi lượt
# sync_hot_marts phía desi chết với:
#     FATAL: password authentication failed for user "chat_ro"
#     DETAIL: Role "chat_ro" does not exist.
#
# Lỗi này ẩn mất 9 tiếng: postgres_fdw cache kết nối remote theo backend, nên các
# backend desi-postgres đang mở vẫn xài kết nối đã xác thực từ TRƯỚC 13:20 và vẫn
# đọc được. Chỉ tới khi desi-order restart lúc 22:23 mở kết nối mới thì 22:37 mới
# lộ ra. Đừng tin "sync vẫn xanh" ngay sau khi dựng lại DB crawler.
#
# Script này chỉ chạy khi cluster init LẦN ĐẦU trên volume rỗng — đúng và chỉ
# đúng cái ca đã giết role.
set -e

# Cố tình FAIL TO, không skip im lặng: init hỏng thì thấy ngay lúc dựng stack,
# còn hơn stack lên xanh rồi ETL phía desi chết âm thầm như lần 22/09.
: "${CHAT_RO_PASSWORD:?CHAT_RO_PASSWORD chưa đặt — khai trong deploy/.env, phải TRÙNG mật khẩu trong USER MAPPING của foreign server chat_crawler bên desi-postgres}"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
     -v pw="$CHAT_RO_PASSWORD" -v db="$POSTGRES_DB" -v owner="$POSTGRES_USER" <<'SQL'
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'chat_ro') THEN
        EXECUTE 'CREATE ROLE chat_ro LOGIN CONNECTION LIMIT 5';
    END IF;
END
$$;

ALTER ROLE chat_ro PASSWORD :'pw';
-- Chặn ghi ở tầng role, không chỉ trông vào việc chỉ GRANT SELECT.
ALTER ROLE chat_ro SET default_transaction_read_only = on;

GRANT CONNECT ON DATABASE :"db" TO chat_ro;
GRANT USAGE ON SCHEMA public TO chat_ro;

-- Lúc initdb chạy thì migration của crawler CHƯA chạy nên public còn rỗng; dòng
-- này là no-op ở lần đầu, chỉ có tác dụng khi chạy lại tay trên cluster đã có dữ liệu.
GRANT SELECT ON ALL TABLES IN SCHEMA public TO chat_ro;

-- Đây mới là dòng làm việc thật: mọi bảng migration tạo SAU đó (chạy bằng chính
-- role :"owner") tự có SELECT cho chat_ro. Rộng hơn runbook cũ (vốn liệt kê tay 6
-- bảng) NHƯNG đúng hơn: partition messages_yYYYYmMM sinh thêm mỗi tháng, danh
-- sách tay không thể theo kịp. chat_ro chỉ đọc, DB crawler chỉ chứa dữ liệu chat.
ALTER DEFAULT PRIVILEGES FOR ROLE :"owner" IN SCHEMA public GRANT SELECT ON TABLES TO chat_ro;
SQL

echo "[initdb] role chat_ro đã sẵn sàng"
