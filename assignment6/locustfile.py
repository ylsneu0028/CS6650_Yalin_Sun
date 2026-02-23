from locust import task, between
from locust.contrib.fasthttp import FastHttpUser

AUTH_HEADERS = {"X-API-Key": "test"}

class ProductSearchUser(FastHttpUser):
    wait_time = between(0, 0.01)

    @task(5)
    def search_category(self):
        self.client.get(
            "/products/search",
            params={"q": "Electronics"},
            headers=AUTH_HEADERS,
            name="/products/search?q=Electronics",
        )

    @task(3)
    def search_brand(self):
        self.client.get(
            "/products/search",
            params={"q": "Alpha"},
            headers=AUTH_HEADERS,
            name="/products/search?q=Alpha",
        )

    @task(2)
    def search_books(self):
        self.client.get(
            "/products/search",
            params={"q": "Books"},
            headers=AUTH_HEADERS,
            name="/products/search?q=Books",
        )