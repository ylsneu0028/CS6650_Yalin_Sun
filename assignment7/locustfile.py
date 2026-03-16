from locust import HttpUser, task, between
import random
import time
import uuid


class OrderUser(HttpUser):
    wait_time = between(0.1, 0.5)  # 100ms to 500ms

    @task
    def create_sync_order(self):
        payload = {
            "order_id": str(uuid.uuid4()),
            "customer_id": random.randint(1, 1000),
            "status": "pending",
            "items": [
                {
                    "id": 1,
                    "name": "keyboard",
                    "quantity": 1,
                    "price": 100
                }
            ],
            "created_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
        }

        self.client.post(
            "/orders/sync",
            json=payload,
            name="POST /orders/sync"
        )