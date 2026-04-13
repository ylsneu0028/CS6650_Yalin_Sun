# ChaosArena run result (pretty-printed JSON)
import json

RESULT_JSON = r"""
{
  "run_id": "20260411T202216Z_d141d630-2215-4401-9d7d-0b3df2ed3877",
  "email": "sun.yalin@northeastern.edu",
  "nickname": "Yalin",
  "contract": "v1-album-store",
  "status": "completed",
  "early_exit": false,
  "score": 162,
  "base_url": "http://alb-20260411182747530700000007-1998167275.us-west-2.elb.amazonaws.com",
  "started_at": "2026-04-11T20:22:16.148839395Z",
  "completed_at": "2026-04-11T20:23:22.601315612Z",
  "scenarios": [
    {
      "name": "S1_HEALTH_CHECK",
      "status": "PASSED",
      "points_awarded": 5,
      "duration_ms": 5,
      "metrics": {
        "duration_ms": 0
      }
    },
    {
      "name": "HEALTH_PROBE_AFTER_S1",
      "status": "PASSED",
      "points_awarded": 0,
      "duration_ms": 2,
      "metrics": {
        "duration_ms": 0
      }
    },
    {
      "name": "S2_ALBUM_CREATE_READ",
      "status": "PASSED",
      "points_awarded": 15,
      "duration_ms": 69,
      "metrics": {
        "duration_ms": 69
      }
    },
    {
      "name": "HEALTH_PROBE_AFTER_S2",
      "status": "PASSED",
      "points_awarded": 0,
      "duration_ms": 1,
      "metrics": {
        "duration_ms": 0
      }
    },
    {
      "name": "S3_PHOTO_UPLOAD_ASYNC",
      "status": "PASSED",
      "points_awarded": 20,
      "duration_ms": 2113,
      "metrics": {
        "duration_ms": 2113
      }
    },
    {
      "name": "HEALTH_PROBE_AFTER_S3",
      "status": "PASSED",
      "points_awarded": 0,
      "duration_ms": 1,
      "metrics": {
        "duration_ms": 0
      }
    },
    {
      "name": "S4_PHOTO_DELETE",
      "status": "PASSED",
      "points_awarded": 10,
      "duration_ms": 2115,
      "metrics": {
        "duration_ms": 2115
      }
    },
    {
      "name": "HEALTH_PROBE_AFTER_S4",
      "status": "PASSED",
      "points_awarded": 0,
      "duration_ms": 1,
      "metrics": {
        "duration_ms": 0
      }
    },
    {
      "name": "S5_LIST_ALBUMS",
      "status": "PASSED",
      "points_awarded": 10,
      "duration_ms": 2231,
      "metrics": {
        "duration_ms": 2231
      }
    },
    {
      "name": "HEALTH_PROBE_AFTER_S5",
      "status": "PASSED",
      "points_awarded": 0,
      "duration_ms": 1,
      "metrics": {
        "duration_ms": 0
      }
    },
    {
      "name": "S6_STRICT_HEALTH",
      "status": "PASSED",
      "points_awarded": 5,
      "duration_ms": 2,
      "metrics": {
        "duration_ms": 0
      }
    },
    {
      "name": "HEALTH_PROBE_AFTER_S6",
      "status": "PASSED",
      "points_awarded": 0,
      "duration_ms": 1,
      "metrics": {
        "duration_ms": 0
      }
    },
    {
      "name": "S7_DOUBLE_DELETE",
      "status": "PASSED",
      "points_awarded": 10,
      "duration_ms": 2125,
      "metrics": {
        "duration_ms": 2125
      }
    },
    {
      "name": "HEALTH_PROBE_AFTER_S7",
      "status": "PASSED",
      "points_awarded": 0,
      "duration_ms": 1,
      "metrics": {
        "duration_ms": 0
      }
    },
    {
      "name": "S8_CONCURRENT_DELETES_SAME_PHOTO",
      "status": "PASSED",
      "points_awarded": 10,
      "duration_ms": 2109,
      "metrics": {
        "duration_ms": 2109
      }
    },
    {
      "name": "HEALTH_PROBE_AFTER_S8",
      "status": "PASSED",
      "points_awarded": 0,
      "duration_ms": 1,
      "metrics": {
        "duration_ms": 0
      }
    },
    {
      "name": "S9_DELETE_BEFORE_COMPLETE",
      "status": "PASSED",
      "points_awarded": 10,
      "duration_ms": 30522,
      "metrics": {
        "duration_ms": 30522
      }
    },
    {
      "name": "HEALTH_PROBE_AFTER_S9",
      "status": "PASSED",
      "points_awarded": 0,
      "duration_ms": 1,
      "metrics": {
        "duration_ms": 0
      }
    },
    {
      "name": "S10_PER_ALBUM_SEQ_NUMBER",
      "status": "PASSED",
      "points_awarded": 15,
      "duration_ms": 678,
      "metrics": {
        "duration_ms": 678
      }
    },
    {
      "name": "HEALTH_PROBE_AFTER_S10",
      "status": "PASSED",
      "points_awarded": 0,
      "duration_ms": 1,
      "metrics": {
        "duration_ms": 0
      }
    },
    {
      "name": "S11_CONCURRENT_CREATES_LOAD",
      "status": "PASSED",
      "points_awarded": 15,
      "duration_ms": 7042,
      "metrics": {
        "duration_ms": 7042,
        "p95_ms": 24,
        "p99_ms": 47
      }
    },
    {
      "name": "HEALTH_PROBE_AFTER_S11",
      "status": "PASSED",
      "points_awarded": 0,
      "duration_ms": 1,
      "metrics": {
        "duration_ms": 0
      }
    },
    {
      "name": "S12_CONCURRENT_PHOTOS_LOAD",
      "status": "PASSED",
      "points_awarded": 7,
      "duration_ms": 4435,
      "metrics": {
        "duration_ms": 4435,
        "p95_ms": 4285,
        "p99_ms": 4356
      }
    },
    {
      "name": "HEALTH_PROBE_AFTER_S12",
      "status": "PASSED",
      "points_awarded": 0,
      "duration_ms": 1,
      "metrics": {
        "duration_ms": 0
      }
    },
    {
      "name": "S13_MIXED_READWRITE_LOAD",
      "status": "PASSED",
      "points_awarded": 15,
      "duration_ms": 3063,
      "metrics": {
        "duration_ms": 3063,
        "p95_ms": 7,
        "p99_ms": 10
      }
    },
    {
      "name": "HEALTH_PROBE_AFTER_S13",
      "status": "PASSED",
      "points_awarded": 0,
      "duration_ms": 1,
      "metrics": {
        "duration_ms": 0
      }
    },
    {
      "name": "S14_MIXED_FULL_LOAD",
      "status": "PASSED",
      "points_awarded": 15,
      "duration_ms": 4585,
      "metrics": {
        "duration_ms": 4585,
        "p95_ms": 13,
        "p99_ms": 22,
        "extra": {
          "meta_score": 10,
          "upload_error_rate_pct": 0,
          "upload_p95_ms": 4169,
          "upload_p99_ms": 4202,
          "upload_score": 5
        }
      }
    },
    {
      "name": "HEALTH_PROBE_AFTER_S14",
      "status": "PASSED",
      "points_awarded": 0,
      "duration_ms": 1,
      "metrics": {
        "duration_ms": 0
      }
    },
    {
      "name": "S15_LARGE_PAYLOAD_LOAD",
      "status": "PASSED",
      "points_awarded": 0,
      "duration_ms": 5294,
      "metrics": {
        "duration_ms": 5294,
        "error_rate": 1,
        "extra": {
          "accept_p95_ms": 0,
          "accept_p99_ms": 0,
          "accept_score": 0,
          "complete_error_rate_pct": 100,
          "complete_p95_ms": 0,
          "complete_p99_ms": 0,
          "complete_score": 0
        }
      }
    }
  ]
}
"""

def data():
    return json.loads(RESULT_JSON)

if __name__ == '__main__':
    print(RESULT_JSON)
