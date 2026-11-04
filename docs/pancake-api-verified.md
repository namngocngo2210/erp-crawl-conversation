# Pancake (pages.fm) API — Verified Contract

> Verified: 2026-06-26  
> Probed with live credentials against `https://pages.fm`  
> All token values are REDACTED. Personal data (names, phones, emails) are REDACTED.

---

## 1. Base URL & Two-API Architecture

The Pancake system exposes **two distinct API namespaces**, each with different roles:

| Namespace | Base URL | Used for |
|---|---|---|
| **Account API** | `https://pages.fm/api/v1` | List pages, generate page tokens |
| **Page API** | `https://pages.fm/api/public_api/v1` | Conversations, messages, tags |

> The `.env` value `PANCAKE_BASE_URL=https://pages.fm/api/public_api/v1` is **partially correct** — it is valid for conversation/message/tag endpoints, but NOT for list-pages or generate-token which live under `/api/v1`.

### Auth Overview

| Endpoint category | Auth param | Where |
|---|---|---|
| List pages | `access_token` | Query string |
| Generate page token | `access_token` | Query string |
| Conversations | `page_access_token` | Query string |
| Messages | `page_access_token` | Query string |
| Tags | `page_access_token` | Query string |

No `Authorization: Bearer` header is used. All auth is via query params.

---

## 2. Endpoint Specifications

### 2.1 List Pages

```
GET https://pages.fm/api/v1/pages?access_token=$PANCAKE_ACCESS_TOKEN
```

**Response shape:**

```json
{
  "success": true,
  "categorized": {
    "activated": [ /* array of page objects */ ],
    "activated_page_ids": ["..."],
    "hidden": [],
    "inactivated": [],
    "nopermission": []
  }
}
```

**Page object fields (relevant fields):**

```json
{
  "id": "945997011940587",
  "name": "DELAX - Thiết Bị Massage Chính Hãng",
  "platform": "facebook",
  "inserted_at": "2026-06-06T01:40:05",
  "is_activated": true,
  "has_active_subscription": true,
  "connected": true,
  "username": null,
  "timezone": "Asia/Ho_Chi_Minh",
  "role_in_page": "ADMINISTER"
}
```

Full page object keys: `active_user_ids, business, connected, custom_page_color, has_active_subscription, id, inserted_at, is_activated, is_silhouette, last_global_id_crawl, name, need_fix_webhook, page_content_sync_group_id, permissions, platform, platform_extra_info, quick_reply_sync_group_id, role_in_page, settings, shop_id, special_feature, tag_sync_group_id, timezone, username, users`

**Notes:**
- The page `id` is a string (can be numeric like `"945997011940587"` for Facebook, or prefixed like `"tts_..."` for TikTok Shop, `"spo_..."` for Shopee, `"zl_..."` for Zalo, `"pzl_..."` for Personal Zalo).
- Platforms seen: `facebook`, `tiktok_shop`, `shopee_official`, `zalo`, `personal_zalo`.
- Only pages with `has_active_subscription: true` can generate page access tokens.
- No pagination on this endpoint — all pages returned in one call.

---

### 2.2 Generate Page Access Token

```
POST https://pages.fm/api/v1/pages/{page_id}/generate_page_access_token?access_token=$PANCAKE_ACCESS_TOKEN
Content-Type: application/json
Body: {}
```

**IMPORTANT:** Must be `POST` with `Content-Type: application/json` header and empty JSON body `{}`. A plain POST without the Content-Type header returns HTTP 500.

**Response:**

```json
{
  "message": "Tạo mới token thành công",
  "page_access_token": "<redacted>",
  "success": true
}
```

**Error response (no subscription):**
```json
{
  "error_code": 121,
  "errors": [{"error_code": 121, "message": "...", "page_id": "..."}],
  "message": "Không tìm thấy gói cước nào cho người dùng này",
  "success": false
}
```

**Error response (no admin permission):**
```json
{
  "error_code": null,
  "message": "Thiếu quyền Admin",
  "success": false
}
```

**Notes:**
- The `public_api/v1` variant of this endpoint returns `"Page not found"` — only `api/v1` works.
- The generated token has no observed expiry in the response. Treat as long-lived but cache with a TTL for safety.
- Token is a JWT with `{"id": "<page_id>", "timestamp": <unix>}` payload (observed pattern).

---

### 2.3 List Conversations

```
GET https://pages.fm/api/public_api/v1/pages/{page_id}/conversations
    ?page_access_token=$PAGE_TOKEN
    &since=<unix_seconds>
    &until=<unix_seconds>
    &page_number=<int>
```

**All four params are REQUIRED.** Missing `since` → `{"message":"since is required!","success":false}`. Missing `page_number` → `{"message":"page_number is required!","success":false}`.

**Response:**

```json
{
  "success": true,
  "total": 554,
  "conversations": [ /* array of conversation objects */ ]
}
```

**Pagination:**
- Page size: **200 conversations per page** (confirmed).
- `total` field is always present and gives the total count.
- No `has_next` field. Detect end by: `page_number * 200 >= total` or when response count < 200.
- Default sort: by `inserted_at` DESC (newest first). To sort by `updated_at`, pass `sort_by=updated_at`. Also tested `sort_by=inserted_at` which works.

**Conversation object keys (verified):**
`assignee_group_id, assignee_histories, assignee_ids, current_assign_users, customer_id, customers, from, has_phone, id, inserted_at, last_sent_by, message_count, page_customer, page_id, post_id, recent_phone_numbers, seen, snippet, tag_histories, tags, type, updated_at`

**Conversation object sample (redacted):**

```json
{
  "id": "945997011940587_27427851286867310",
  "type": "INBOX",
  "page_id": "945997011940587",
  "inserted_at": "2026-06-14T02:00:56.667139",
  "updated_at": "2026-06-14T02:03:04.381000",
  "snippet": "[preview of last message...]",
  "message_count": 3,
  "seen": true,
  "has_phone": false,
  "tags": [
    {
      "id": 46,
      "text": "TEST MESS",
      "color": "#bd2727",
      "lighten_color": "rgba(189,39,39,0.4)",
      "is_lead_event": false
    }
  ],
  "customers": [
    {
      "id": "f49dd6f6-d775-4bbb-983f-683c3d0137ee",
      "fb_id": "<redacted>",
      "name": "<redacted>"
    }
  ],
  "customer_id": "<redacted>",
  "page_customer": {
    "id": "f49dd6f6-d775-4bbb-983f-683c3d0137ee",
    "name": "<redacted>",
    "inserted_at": "2026-06-14T02:00:54",
    "customer_id": "<redacted>",
    "psid": "<redacted>",
    "gender": null,
    "global_id": null,
    "birthday": null,
    "notes": null,
    "recent_orders": null
  },
  "from": {
    "id": "<redacted_fb_id>",
    "name": "<redacted>"
  },
  "last_sent_by": {
    "admin_id": "945997011940587",
    "admin_name": "Botcake",
    "id": "945997011940587",
    "name": "DELAX - Thiết Bị Massage Chính Hãng"
  },
  "post_id": null,
  "assignee_ids": [],
  "assignee_group_id": null,
  "recent_phone_numbers": [],
  "assignee_histories": [],
  "current_assign_users": [],
  "tag_histories": []
}
```

**Conversation types observed:** `"INBOX"`, `"COMMENT"`

**Time filter semantics:**
- `since` and `until` are **Unix timestamps in seconds** (integer).
- They filter by `inserted_at` (creation time), NOT by `updated_at`.
- To get all conversations updated recently, use a wide `since` window and sort by `updated_at`.

---

### 2.4 List Messages

```
GET https://pages.fm/api/public_api/v1/pages/{page_id}/conversations/{conversation_id}/messages
    ?page_access_token=$PAGE_TOKEN
    &current_count=<int>   (offset for pagination — see §2.4.1; default 0)
```

**Response shape (rich envelope — many fields):**

```json
{
  "success": true,
  "conversation_id": "945997011940587_27652135287726020",
  "messages": [ /* array of message objects, max 30 */ ],
  "customers": [ /* array of customer objects for this conversation */ ],
  "gender": "female",
  "birthday": null,
  "lives_in": "Thanh Hóa",
  "global_id": "100092175918383",
  "app": "53",
  "notes": null,
  "activities": [],
  "recent_orders": null
  /* ... many more UI/profile fields ... */
}
```

**Message Pagination — see dedicated section "Message pagination — verified" (§2.4.1) below.** The working mechanism is the `current_count` offset param. `page_number` does NOT work for messages.

**Message object keys (verified):**
`attachments, can_comment, can_hide, can_like, can_remove, can_reply_privately, comment_count, conversation_id, edit_history, from, has_phone, id, inserted_at, is_hidden, is_livestream_order, is_parent, is_parent_hidden, is_removed, like_count, message, message_tags, original_message, page_id, parent_id, phone_info, private_reply_conversation, removed_by, rich_message, seen, show_info, type, user_likes`

**Message object sample (redacted):**

```json
{
  "id": "m_5S-h1Le2zA4mbSzIgPzyDxD8eJ_...",
  "message": "<div>Nay có bán hàng k shop</div>",
  "original_message": "Nay có bán hàng k shop",
  "type": "INBOX",
  "seen": false,
  "from": {
    "id": "<redacted_fb_id>"
  },
  "inserted_at": "2026-06-14T02:00:49.000000",
  "page_id": "945997011940587",
  "conversation_id": "945997011940587_27427851286867310",
  "attachments": [],
  "is_removed": false,
  "is_hidden": false,
  "is_parent": false,
  "parent_id": null,
  "rich_message": null,
  "private_reply_conversation": null,
  "removed_by": null,
  "comment_count": null,
  "like_count": null
}
```

**Message with attachments sample:**

```json
{
  "id": "m_hXEgaNs8tin-u1ntqCaA...",
  "type": "INBOX",
  "inserted_at": "2026-06-10T02:18:57.224000",
  "attachments": [
    {
      "type": "photo",
      "url": "https://content.pancake.vn/2-2606/2026/6/4/<redacted>.jpg",
      "image_data": {"height": 1152, "width": 2047}
    },
    {
      "type": "photo",
      "url": "https://content.pancake.vn/2-2606/2026/6/4/<redacted>.jpg",
      "image_data": {"height": 800, "width": 800}
    }
  ]
}
```

### 2.4.1 Message pagination — VERIFIED (retrieving ALL messages)

**Working mechanism: the `current_count` query param (offset by message count).**

```
GET .../conversations/{conv_id}/messages
    ?page_access_token=$PAGE_TOKEN
    &current_count=<number_of_messages_already_fetched>
```

**Semantics (verified against a 76-message and a 55-message conversation):**
- Each call returns up to **30 messages**, ordered **ascending by `inserted_at`** within the response (index 0 = oldest of the window, last index = newest of the window).
- `current_count=0` returns the **newest** 30 messages.
- `current_count=30` returns the **next 30 older** messages, contiguous with the previous window (the newest message of this window is exactly one step older than the oldest message of the previous window — **no gap, no overlap**).
- Increase `current_count` by the number of messages already collected; when a call returns an array shorter than 30, that window is the last partial page; the **next** call returns an **empty `messages` array**, signaling the end.

**Exact recipe to fetch ALL messages of a conversation:**

```
collected = 0
loop:
    resp = GET messages?page_access_token=PT&current_count={collected}
    if resp.messages is empty: break       # reached the start of the conversation
    append resp.messages to result
    collected += len(resp.messages)         # advance by ACTUAL returned count
```

> Advance by the **actual returned count**, not a hardcoded 30, so a short middle page (rare) cannot cause a skip. Stepping by the returned length yielded **0 duplicates and 0 gaps** in verification.

**Verification evidence (redacted):**

| Conversation | message_count (from conv list) | Pages walked (`current_count`) | Per-page counts | Total unique collected | Duplicates |
|---|---|---|---|---|---|
| A (big) | 76 | 0, 30, 60, 90 | 30, 30, 17, 0 | **77** | 0 |
| B | 55 | 0, 30, 56 | 30, 26, 0 | **56** | 0 |

(The collected total can slightly exceed the conversation's reported `message_count` because the count field excludes some system/auxiliary message rows; every returned `id` was unique.)

**Params that DO NOT work (tested and rejected):**

| Param | Result |
|---|---|
| `page_number` | Ignored — always returns the latest 30, identical for any value |
| `count`, `page_size` | Ignored — window stays ≤30 |
| `limit` | Ignored — does not enlarge window (returned 25/30, not 100) |
| `before`, `last_message_id`, `until_id` (= oldest msg id) | Ignored — returns the same latest 30 |
| `after`, `from_message_id` | Cause HTTP 500 `"Server internal error"` |
| `since`, `until` (Unix seconds) | Ignored on the messages endpoint — always returns latest 30 (unlike conversations, where they ARE required and functional) |

**Paging metadata:** The messages response has NO `total`, `paging`, `cursor`, `next`, `has_more`, or `older` key. End-of-data is detected solely by an empty `messages` array. To know roughly how many messages exist up front, use the conversation object's **`message_count`** field from the conversations list (approximate; use the empty-array terminator as the authoritative stop condition).

**Conclusion:** Full message backfill IS achievable via `current_count`. The project's core requirement (retrieve ALL messages of every conversation) is satisfied.

---

**Direction (inbound vs outbound):** No explicit `direction` field. Infer from `from.id`:
- `from.id == page_id` → **outbound** (sent by page/admin).
- `from.id != page_id` → **inbound** (sent by customer).

**`inserted_at` format:** ISO 8601 with microseconds: `"2026-06-14T02:00:49.000000"` (no timezone suffix — treat as UTC based on Pancake's server timezone).

**Customer info in messages response:**
The `customers` array in the messages response is richer than in the conversations list:

```json
{
  "id": "f49dd6f6-d775-4bbb-983f-683c3d0137ee",
  "customer_id": "<redacted>",
  "fb_id": "<redacted>",
  "name": "<redacted>",
  "personal_info": {"gender": "male"},
  "global_id": null,
  "is_followed": null,
  "can_inbox": true,
  "recent_phone_numbers": ["<redacted>"],
  "web_sources": [],
  "ad_clicks": [],
  "address_source": []
}
```

Key: `recent_phone_numbers` is in the messages response's customer object (not in the conversations list customer object).

---

### 2.5 List Tags

```
GET https://pages.fm/api/public_api/v1/pages/{page_id}/tags
    ?page_access_token=$PAGE_TOKEN
```

No additional params required.

**Response:**

```json
{
  "tags": [
    {
      "id": 22,
      "text": "BOT-BNC02",
      "color": "#323da9",
      "lighten_color": "rgba(50,61,169,0.4)",
      "is_lead_event": false
    },
    {
      "id": 23,
      "text": "BOT-DC30",
      "color": "#1cae67",
      "description": "BOT-DC30",
      "lighten_color": "rgba(28,174,103,0.4)",
      "is_lead_event": false
    }
  ]
}
```

**Tag object fields:** `id` (integer), `text` (tag name), `color` (hex), `lighten_color` (rgba string), `is_lead_event` (bool), optionally `description`.

**Notes:**
- Tag `id` is an **integer**, not a string.
- Tags embedded in conversations (`conversation.tags[]`) have the same shape.
- No `name` field — the tag label is `text`.

---

## 3. Rate Limiting

- No `X-RateLimit-*` or `Retry-After` headers observed.
- 5 rapid sequential requests to the same endpoint all returned 200 OK.
- No 429 responses observed during testing.
- The plan's assumption of needing a rate limiter is prudent but the actual limit values are unknown. Conservative default of 5 RPS per page is reasonable.

---

## 4. Customer Info — Where It Lives

| Location | Fields available |
|---|---|
| `conversations[].customers[]` | `id` (UUID), `fb_id`, `name` |
| `conversations[].page_customer` | `id`, `name`, `inserted_at`, `customer_id`, `psid`, `gender`, `global_id`, `birthday`, `notes`, `recent_orders` |
| `messages_response.customers[]` | `id`, `customer_id`, `fb_id`, `name`, `personal_info.gender`, `global_id`, `is_followed`, `can_inbox`, `recent_phone_numbers`, `web_sources`, `ad_clicks`, `address_source` |
| `messages_response.gender` | top-level gender field |
| `messages_response.global_id` | top-level Facebook global ID |

No separate `/customers` endpoint exists — customer data is always embedded in conversation/message responses.

---

## 5. Reconciliation with Plan Assumptions (spec §8)

### 5.1 Base URL

| Assumed | Actual | Impact |
|---|---|---|
| Single base URL `https://pages.fm/api/public_api/v1` | **Two URLs needed**: `api/v1` for pages+tokens, `public_api/v1` for conversations/messages/tags | Client must use different base URLs per operation |

### 5.2 Field Name Corrections (assumed → actual)

| Entity | Assumed field name | Actual field name | Notes |
|---|---|---|---|
| **Page** | `id` (string) | `id` (string) | ✓ Same |
| **Page** | `name` | `name` | ✓ Same |
| **Page** | `platform` | `platform` | ✓ Same |
| **Page** | *(no assumption)* | Response is under `categorized.activated[]` | Plan assumed `data[]` — WRONG |
| **Tag** | `id` | `id` (**integer**, not string) | Type differs |
| **Tag** | `name` | `text` | **WRONG** — must use `text` |
| **Tag** | `color` | `color` | ✓ Same |
| **Conversation** | `id` | `id` | ✓ Same |
| **Conversation** | `updated_at` | `updated_at` | ✓ Same |
| **Conversation** | `type` | `type` | ✓ Same (values: `"INBOX"`, `"COMMENT"`) |
| **Conversation** | `snippet` | `snippet` | ✓ Same |
| **Conversation** | `message_count` | `message_count` | ✓ Same |
| **Conversation** | *(assumed last_message_at)* | No `last_message_at` in conversation — use `updated_at` as proxy | Missing field |
| **Conversation** | tags as `tag_ids []string` | Tags embedded as full objects in `tags[]` with `id`(int), `text`, `color` | Structure differs |
| **Customer (in conv)** | `id` | `id` (UUID string) | ✓ Same |
| **Customer (in conv)** | `name` | `name` | ✓ Same |
| **Customer (in conv)** | `fb_id` | `fb_id` | ✓ Same |
| **Customer (in conv)** | `phone` | `recent_phone_numbers[]` (in messages response) | Different field, different shape |
| **Customer (in conv)** | `email` | `recent_emails[]` (in messages response top level) | Different field |
| **Customer (in conv)** | `gender` | `personal_info.gender` (nested) or `gender` (top-level in messages response) | Nested in customer, top-level in response |
| **Message** | `id` | `id` | ✓ Same (long opaque string for FB) |
| **Message** | `inserted_at` | `inserted_at` | ✓ Same |
| **Message** | `sender_id` | `from.id` (nested) | Different structure |
| **Message** | `direction` (in/out) | No field — infer from `from.id == page_id` | Must compute |
| **Message** | `type` | `type` | ✓ Same |
| **Message** | `content` | `message` (contains HTML) or `original_message` (plain text) | Field name differs |
| **Message** | `attachments` | `attachments[]` with `type`, `url`, `image_data` | ✓ Same field name, structure is array of objects |
| **GenerateToken** | Response: `page_access_token` | `page_access_token` | ✓ Same |

### 5.3 Pagination Corrections

| Parameter | Assumed | Actual |
|---|---|---|
| Conversations | `page_number` (int) | `page_number` (int) ✓ — **REQUIRED** |
| Conversations | `since`/`until` (unix seconds) | `since`/`until` (unix seconds) ✓ — **REQUIRED** |
| Conversations | `has_next` response field | **No `has_next`** — use `total` field to compute: `has_more = (page_number * page_size) < total` |
| Conversations page size | Unknown | 200 per page |
| Messages | `page_number` (int) | **Non-functional** — use `current_count` (offset) instead; see §2.4.1 |
| Messages pagination | (assumed `page_number`) | **`current_count`** = number of messages already fetched; step by returned count; empty array = end |
| Messages | `has_next` response field | **Not present** — detect end by empty `messages` array |
| Messages page size | Unknown | 30 per page; walk older history via `current_count` |

### 5.4 Auth Endpoint Correction

| Assumed | Actual |
|---|---|
| `GET /pages/{id}/generate_page_access_token` or `POST` without Content-Type | `POST /pages/{id}/generate_page_access_token` with `Content-Type: application/json` body `{}` **required** |

---

## 6. Sprint 2 DTO & Client Corrections Needed

### `internal/dto/pancake.go` changes required:

1. **`PageDTO`**: Add `InsertedAt time.Time`. The list response is under `categorized.activated`, NOT `data`.
2. **`TagDTO`**: Change `Name string` → `Name string` (mapped from API's `text` field), and `ID string` → `ID int` (integer).
3. **`ConversationDTO`**: 
   - Remove `LastMessageAt` (no such field in API — use `UpdatedAt`).
   - Change `TagIDs []string` → `Tags []TagDTO` (full embedded objects).
   - Add `InsertedAt time.Time`.
4. **`MessageDTO`**:
   - Change `SenderID string` → populate from `from.id`.
   - Change `Direction string` → computed field (not from API).
   - Change `Content string` → map from `message` (HTML) or `original_message` (plain).
5. **`ListParams`**: Keep `PageNumber int`, `Since *time.Time`, `Until *time.Time` — these map correctly to the actual params.
6. **`ConversationsResponse`**: Change `HasNext bool` → keep for internal use but compute from `total` field.
7. Add **`PagesResponse`** struct wrapping `categorized.activated`.

### `internal/pancake/client.go` changes required:

1. `ListPages` must call `https://pages.fm/api/v1/pages` (not the public_api base URL).
2. `GeneratePageToken` must call `https://pages.fm/api/v1/pages/{id}/generate_page_access_token` as **POST** with `Content-Type: application/json` body `{}`.
3. `ListConversations` — `since` and `until` are **required**, and `page_number` is **required** (non-zero). The response total-based `has_next` computation: `resp.HasNext = (params.PageNumber * 200) < raw.Total`.
4. `ListMessages` — paginate with the **`current_count`** offset param (NOT `page_number`). Loop: start `current_count=0`, append `resp.messages`, increment `current_count` by the actual returned length, stop when `messages` is empty. This retrieves the FULL message history (verified — see §2.4.1). The `dto.ListParams` for messages should carry an offset/`CurrentCount` field rather than `PageNumber`; signature stays otherwise unchanged.
5. `ListTags` — works correctly with `page_access_token` query param.

---

## 7. Observed Errors Reference

| Error | Meaning |
|---|---|
| `{"error_code": 102, "message": "Invalid access_token"}` | `page_access_token` is invalid or expired |
| `{"error_code": 121, "message": "Không tìm thấy gói cước..."}` | Page has no active subscription |
| `{"message": "Thiếu quyền Admin", "success": false}` | Caller does not have admin role on this page |
| `{"message": "since is required!", "success": false}` | Missing `since` param in conversations |
| `{"message": "page_number is required!", "success": false}` | Missing `page_number` param in conversations |
| `"Server internal error"` (plain string) | Usually caused by wrong Content-Type or unsupported endpoint path |
| HTML response (200 OK) | Wrong base URL or path — the SPA returns HTML for unknown routes without Content-Type: application/json |

---

## 8. Conversations cursor pagination (last_conversation_id) — VERIFIED

> Verified: 2026-06-26 against page_id `518854254652984` (highest-traffic activated Facebook page; `total=2738` conversations in a 30-day window).

### Default page size (no params)

Calling `GET .../conversations?page_access_token=PT` with **no other params** returns:

```json
{"message": "since is required!", "success": false}
```

The endpoint has **no parameter-free mode**. The minimum required set is:

```
since=<unix_seconds>   (REQUIRED)
until=<unix_seconds>   (REQUIRED — must be within 30 days of since)
page_number=<int>      (REQUIRED — must be >= 1)
```

Default page size when all required params are supplied: **200 conversations per call** (observed across all pages). Total is always present in the response: `{"total": 2738, "conversations": [...]}`.

### Whether `since`/`until` are required when using `last_conversation_id`

**Yes — `since` and `until` are ALWAYS required**, regardless of whether `last_conversation_id` is present.

Tested combinations:

| Params used | Result |
|---|---|
| `last_conversation_id` only | `{"message": "since is required!", "success": false}` |
| `last_conversation_id` + `since` only | `{"message": "until is required!", "success": false}` |
| `last_conversation_id` + `since` + `until` | `{"message": "page_number is required!", "success": false}` |
| `last_conversation_id` + `since` + `until` + `page_number` | `success: true`, 200 conversations |

### Critical finding: `last_conversation_id` is IGNORED

The parameter `last_conversation_id` has **no observable effect** on the response. Verified by three parallel calls all returning identical results:

1. `page_number=2` + correct `last_conversation_id` (real last ID from page 1)
2. `page_number=2` + fake/nonexistent `last_conversation_id`
3. `page_number=2` + no `last_conversation_id`

All three returned the **exact same 200 conversations** with **identical first and last IDs**.

**Conclusion:** The `last_conversation_id` parameter is either completely ignored by the server or is not implemented for this endpoint. Pagination is entirely driven by `page_number`.

### Exact recipe to walk ALL conversations (pseudocode)

Pagination is pure offset-by-page-number within a fixed `since`/`until` window. To backfill all conversations across a long historical range, chunk time into 30-day windows and page through each:

```pseudocode
PAGE_SIZE = 200

function fetch_all_conversations(page_id, page_token, start_ts, end_ts):
    all_conversations = []
    
    # Chunk into 30-day windows (max allowed range)
    window_start = start_ts
    while window_start < end_ts:
        window_end = min(window_start + 30*86400, end_ts)
        
        # Page through all pages in this window
        page_number = 1
        while true:
            resp = GET /conversations
                ?page_access_token=page_token
                &since=window_start
                &until=window_end
                &page_number=page_number
            
            if not resp.success:
                handle_error(resp.message)
                break
            
            convs = resp.conversations
            if len(convs) == 0:
                break   # exhausted this window
            
            all_conversations.extend(convs)
            
            total = resp.total
            if page_number * PAGE_SIZE >= total:
                break   # reached last page
            
            page_number += 1
            sleep(rate_limit_delay)  # respect 5 RPS per page
        
        window_start = window_end
    
    return all_conversations
```

### Termination condition

Two equivalent termination signals (use either or both as redundancy):

1. **Count-based (preferred):** `page_number * PAGE_SIZE >= total` — `total` is always returned in the response and is stable within a window.
2. **Empty-array:** Response `conversations` array has length 0. Verified: page 15 when total=2738 (ceil(2738/200)=14 pages) returned `{"total": 2738, "conversations": [], "success": true}`.

Both conditions are safe to check. The count-based check avoids an unnecessary extra API call; the empty-array check is the authoritative fallback.

**Observed on test page (total=2738, 30-day window):**

| Page | Count | Notes |
|---|---|---|
| 1 | 200 | Full page |
| 2 | 200 | Full page, zero overlap with page 1 |
| 3 | 200 | Full page, zero overlap with pages 1-2 |
| 4 | 200 | Full page, zero overlap with pages 1-3 |
| 5 | 200 | Full page, zero overlap with pages 1-4 |
| 14 | 138 | Last partial page (2738 mod 200 = 138) |
| 15 | 0 | Empty — termination signal |

### Sort order

Conversations are sorted **newest-first by `updated_at`** (DESC). Verified:

- Page 1, index 0: `updated_at=2026-06-26T03:05:56` (most recent)
- Page 1, index 199: `updated_at=2026-06-24T14:37:36` (older)
- `[DateTime]::Parse(first.updated_at) > [DateTime]::Parse(last.updated_at)` → `True`

Both `inserted_at` and `updated_at` fields are always present on every conversation object.

### Field presence (all fields verified on live data)

| Field | Present | Null-free |
|---|---|---|
| `id` | True | True |
| `type` | True | True |
| `snippet` | True | True |
| `message_count` | True | True |
| `updated_at` | True | True |
| `inserted_at` | True | True |
| `tags` | True | True (may be empty array) |
| `customers` | True | True (may be empty array) |
| `page_customer` | True | True |

Full conversation object keys confirmed: `id, type, tags, seen, from, snippet, inserted_at, updated_at, message_count, page_id, assignee_ids, last_sent_by, assignee_group_id, customers, has_phone, post_id, recent_phone_numbers, customer_id, page_customer, ads, ad_ids, assignee_histories, current_assign_users, tag_histories`

### Comparison: cursor (`last_conversation_id`) vs `page_number` + window approach

| Aspect | `last_conversation_id` cursor | `page_number` + `since`/`until` window |
|---|---|---|
| **Works?** | No — parameter is ignored | Yes — fully functional |
| **Required params** | since + until + page_number (last_id ignored) | since + until + page_number |
| **No-overlap guarantee** | N/A (cursor does nothing) | Yes — zero overlap observed across 5 consecutive pages |
| **Stable during crawl?** | N/A | Yes — `total` is stable within a fixed window |
| **Full backfill strategy** | Not possible via cursor | Yes — chunk into ≤30-day windows, loop page_number 1..ceil(total/200) |
| **Recommendation** | Do NOT rely on `last_conversation_id` | Use `page_number` + fixed time windows |

**Recommendation for full historical backfill:**

Use the `page_number` + sliding `since`/`until` window approach. Chunk historical time into 30-day periods (the maximum allowed range per request), and for each period loop `page_number` from 1 until `page_number * 200 >= total` or the returned array is empty. This approach has zero overlap between pages, stable `total`, and no dependency on a cursor parameter that the API silently ignores.
