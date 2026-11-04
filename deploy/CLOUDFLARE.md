# Triển khai chat.desiglobal.vn qua Cloudflare

Kiến trúc:

```
Browser ──HTTPS──> Cloudflare (Full strict) ──HTTPS:443──> host nginx (Origin Cert)
                                                              └─> 127.0.0.1:18081 (container web, nginx)
                                                                     └─/api─> crawler:8080 (mạng nội bộ docker)
```

Edge công khai duy nhất là host nginx ở 80/443. Postgres/Redis/Crawler/Web chỉ bind `127.0.0.1`, không lộ ra internet.

## 1. Cloudflare Dashboard

1. **DNS** → thêm bản ghi A: `chat` → `<IP server>`, Proxy status = **Proxied** (đám mây cam).
2. **SSL/TLS → Overview** → chọn mode **Full (strict)**.
3. **SSL/TLS → Origin Server → Create Certificate**:
   - Hostnames: `chat.desiglobal.vn` (có thể thêm `*.desiglobal.vn`).
   - Lưu phần **Origin Certificate** và **Private Key**.
4. (Khuyến nghị) **SSL/TLS → Edge Certificates** → bật **Always Use HTTPS**.

## 2. Cài cert lên server

```bash
sudo mkdir -p /etc/ssl/cloudflare
sudo nano /etc/ssl/cloudflare/chat.desiglobal.vn.pem   # dán Origin Certificate
sudo nano /etc/ssl/cloudflare/chat.desiglobal.vn.key   # dán Private Key
sudo chmod 600 /etc/ssl/cloudflare/chat.desiglobal.vn.key
```

(Tùy chọn) Bật **Authenticated Origin Pulls** để chỉ Cloudflare gọi được origin: tải CA
từ Cloudflare về `/etc/ssl/cloudflare/authenticated_origin_pull_ca.pem` rồi bỏ comment 2 dòng
`ssl_client_certificate` / `ssl_verify_client` trong file conf.

## 3. Cài nginx host + nạp cấu hình

```bash
sudo apt install -y nginx
sudo cp deploy/nginx/chat.desiglobal.vn.conf /etc/nginx/conf.d/
sudo nginx -t && sudo systemctl reload nginx
```

## 4. Chạy stack + firewall

```bash
docker compose -f deploy/docker-compose.yml up -d --build
sudo ufw allow 80,443/tcp        # chỉ mở edge; KHÔNG mở 18081/18080/15432/16379
```

## 5. Cấu hình app

- Trong `.env` (dùng cho compose) đặt:
  ```
  CORS_ORIGIN=https://chat.desiglobal.vn
  ```
  (FE và API chung origin qua nginx nên trình duyệt không kích hoạt CORS, nhưng set cụ thể vẫn an toàn hơn `*`.)

## Kiểm tra

```bash
curl -I https://chat.desiglobal.vn            # 200, header Server: cloudflare
curl https://chat.desiglobal.vn/api/v1/healthz
```

Nếu lỗi **Error 521/522**: host nginx chưa chạy hoặc firewall chặn 443.
Nếu **redirect loop**: kiểm tra đang để Full (strict), không phải Flexible.
