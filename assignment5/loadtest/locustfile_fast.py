from locust import FastHttpUser, task, between
import random
import time

class ProductApiUser(FastHttpUser):
    wait_time = between(0.1, 0.3) 
    host = "http://16.148.47.20:8080"

    def on_start(self):
        self.headers = {
            "X-API-Key": "test",
            "Content-Type": "application/json"
        }

    @task(3)  # GET : POST = 3 : 1
    def get_product(self):
        product_id = random.randint(1, 5)
        self.client.get(
            f"/products/{product_id}",
            headers=self.headers,
            name="GET /products/:id"
        )

    @task(1)
    def post_product_details(self):
        product_id = random.randint(1, 5)
        payload = {
            "product_id": product_id,
            "sku": f"SKU-{product_id}",
            "manufacturer": "LocustTest",
            "category_id": 1,
            "weight": 1000,
            "some_other_id": int(time.time())
        }

        self.client.post(
            f"/products/{product_id}/details",
            json=payload,
            headers=self.headers,
            name="POST /products/:id/details"
        )
