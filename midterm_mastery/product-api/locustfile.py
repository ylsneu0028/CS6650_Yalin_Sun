from locust import FastHttpUser, task, between

class ProductSearchUser(FastHttpUser):
    """
    Locust user for Assignment 7:
    - Uses FastHttpUser (as required in Assignment 6)
    - Sends authenticated search requests
    - Minimal wait time to stress the service
    """

    # Very small wait time → aggressive load
    wait_time = between(0.01, 0.05)

    def on_start(self):
        # Authentication headers (any non-empty value is accepted)
        self.headers = {
            "X-API-Key": "test"
        }

    @task
    def search_product(self):
        # Common query ensures consistent behavior
        self.client.get(
            "/products/search?q=alpha",
            headers=self.headers,
            name="/products/search"
        )