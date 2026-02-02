from locust import FastHttpUser, task, between
import time
import random

class AlbumUser(FastHttpUser):
    # Simulate a small user think time (avoid tight loop)
    wait_time = between(0.2, 0.5)

    @task(3)
    def get_albums(self):
        # Read the full album list
        self.client.get("/albums", name="GET /albums")

    @task(1)
    def post_album(self):
        # Create a unique-ish ID to reduce accidental duplicates
        new_id = str(int(time.time() * 1000)) + str(random.randint(0, 999))
        payload = {
            "id": new_id,
            "title": "LoadTest Album",
            "artist": "Locust",
            "price": 9.99
        }
        self.client.post("/albums", json=payload, name="POST /albums")


