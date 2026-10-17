# Definition of done

A box is **Met** only when the code behaves that way and a test asserts it. **Partial** means the code does it, but no test asserts that scenario. **Unmet** means it is not implemented.

Playback storyboard: `openspec/gifs/playback-flow.gif`.

## Met

| Done when | Evidence |
| --- | --- |
| Create stores a PENDING payment and returns `payment_id` | Handler test and playback. |
| Get returns that payment | Handler test. Playback checks a get after replay. |
| A reused idempotency key is HTTP 400 with the original id | Handler test and playback. Count stays 1000. |
| An unknown id is HTTP 404 `payment not found` | Handler test. |
| The store can mark a known payment SUCCEEDED and rejects an unknown id | Repository test. Not an HTTP route. |
| `DATABASE_URL` selects memory or Postgres with the same create, read, and idempotency rules | `cmd/server` and `TestPostgresPayment`. |
| 1000 creates finish in under 60 seconds, with one HTTP 503 retried on each call | Playback test, 79ms. |
| HTTP 400 is not retried. 5xx is retried up to the attempt limit | Playback unit tests. |
| Statement coverage is measured | 46.0%. |

## Partial

| Done when | Why it is not Met |
| --- | --- |
| Invalid JSON on create is HTTP 400 | Implemented. No test sends a bad body. |
| A non-UUID payment id is HTTP 400 | Implemented. No test requests one. |

## Unmet

| Not done | Why |
| --- | --- |
| Cancel, refund, and list | Named in the README. No HTTP route. |
| Status change on the API | `UpdateStatus` is repository-only. |
| Coverage at a high bar | 46.0% includes untested `cmd/server` and skipped paths. No 80% gate. |

## GIF review

Good enough. Retry frame stays at 0 payments. Playback frame is HTTP 400 and still 1000 pending. Storyboard, not a screen capture.
